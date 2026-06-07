package runtime

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ── Tool definitions (OpenAI-compatible schema) ──────────────────────────────

// ToolDef describes a tool that can be offered to the LLM.
type ToolDef struct {
	Type     string      `json:"type"` // always "function"
	Function ToolFuncDef `json:"function"`
}

type ToolFuncDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolResult is broadcast to the UI for display.
type ToolResult struct {
	CallID string `json:"call_id"`
	Name   string `json:"name"`
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// ── Tool provider abstraction ────────────────────────────────────────────────

// ToolProvider supplies the set of tools available to a single chat request and
// executes them. This lets each request (assistant chat, coding agent, MCP-enabled
// session, skills) expose a different toolset without a coarse global registry.
type ToolProvider interface {
	// Tools returns the OpenAI-schema tool definitions offered to the model.
	Tools() []ToolDef
	// Execute runs the named tool with JSON arguments and returns a result string.
	Execute(name, argsJSON string) (string, error)
}

// builtinProvider is the default provider backed by BuiltinTools/ExecuteTool.
type builtinProvider struct{}

func (builtinProvider) Tools() []ToolDef                          { return BuiltinTools() }
func (builtinProvider) Execute(name, args string) (string, error) { return ExecuteTool(name, args) }

// DefaultToolProvider returns the builtin tool provider.
func DefaultToolProvider() ToolProvider { return builtinProvider{} }

// CompositeProvider merges several providers into one, concatenating their tools
// and dispatching Execute to whichever provider declares the named tool.
type CompositeProvider struct {
	providers []ToolProvider
}

// NewCompositeProvider builds a provider from the given (non-nil) providers.
func NewCompositeProvider(ps ...ToolProvider) *CompositeProvider {
	var out []ToolProvider
	for _, p := range ps {
		if p != nil {
			out = append(out, p)
		}
	}
	return &CompositeProvider{providers: out}
}

func (c *CompositeProvider) Tools() []ToolDef {
	var all []ToolDef
	for _, p := range c.providers {
		all = append(all, p.Tools()...)
	}
	return all
}

func (c *CompositeProvider) Execute(name, args string) (string, error) {
	for _, p := range c.providers {
		for _, t := range p.Tools() {
			if t.Function.Name == name {
				return p.Execute(name, args)
			}
		}
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// ── Built-in tool catalog ────────────────────────────────────────────────────

// BuiltinTools returns the list of tool definitions available for server-side tool-use.
func BuiltinTools() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "get_current_time",
				Description: "Returns the current date and time with timezone.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"format":{"type":"string","description":"Optional Go time layout string. Default is RFC3339."}},"required":[]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "calculator",
				Description: "Evaluates a mathematical expression. Supports +, -, *, /, parentheses, sqrt, pow, abs, sin, cos, tan, log, exp, floor, ceil, round, max, min, pi, e.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"expression":{"type":"string","description":"The math expression, e.g. '(3+5)*2' or 'sqrt(144)'"}},"required":["expression"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "web_search",
				Description: "Searches the web for current information, news, or facts.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"The search query"}},"required":["query"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "read_file",
				Description: "Reads the contents of a file from the local filesystem.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Absolute or relative path to the file"}},"required":["path"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "write_file",
				Description: "Writes content to a file on the local filesystem. Creates the file if it does not exist.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Path to the file"},"content":{"type":"string","description":"Content to write"}},"required":["path","content"]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "list_directory",
				Description: "Lists files and directories at the given path.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Directory path to list. Defaults to current directory.","default":"."}},"required":[]}`),
			},
		},
		{
			Type: "function",
			Function: ToolFuncDef{
				Name:        "fetch_url",
				Description: "Fetch a web page or document by URL and return its text content (HTML is stripped to readable text). Use to open and read links the user provides.",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"The full URL to fetch, e.g. https://example.com/page"}},"required":["url"]}`),
			},
		},
	}
}

// ── Tool executor ────────────────────────────────────────────────────────────

// ExecuteTool runs the named tool with the given JSON arguments and returns a result string.
func ExecuteTool(name string, argsJSON string) (string, error) {
	switch name {
	case "get_current_time":
		return toolGetCurrentTime(argsJSON)
	case "calculator":
		return toolCalculator(argsJSON)
	case "web_search":
		return toolWebSearch(argsJSON)
	case "read_file":
		return toolReadFile(argsJSON)
	case "write_file":
		return toolWriteFile(argsJSON)
	case "list_directory":
		return toolListDirectory(argsJSON)
	case "fetch_url":
		return toolFetchURL(argsJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}

// ── fetch_url ────────────────────────────────────────────────────────────────

func toolFetchURL(argsJSON string) (string, error) {
	var args struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	u := strings.TrimSpace(args.URL)
	if u == "" {
		return "", fmt.Errorf("url is required")
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "https://" + u
	}
	client := &http.Client{Timeout: 25 * time.Second}
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Vortelio)")
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not fetch: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // cap 2MB
	text := htmlToText(string(body))
	if len(text) > 8000 {
		text = text[:8000] + "\n…[contenuto troncato]"
	}
	out := map[string]interface{}{"url": u, "status": resp.StatusCode, "content": text}
	b, _ := json.Marshal(out)
	return string(b), nil
}

var (
	reScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	reTags        = regexp.MustCompile(`(?s)<[^>]+>`)
	reWS          = regexp.MustCompile(`[ \t]+`)
	reBlankLines  = regexp.MustCompile(`\n\s*\n\s*\n+`)
)

func htmlToText(h string) string {
	h = reScriptStyle.ReplaceAllString(h, " ")
	h = reTags.ReplaceAllString(h, " ")
	h = htmlUnescape(h)
	h = reWS.ReplaceAllString(h, " ")
	h = reBlankLines.ReplaceAllString(h, "\n\n")
	return strings.TrimSpace(h)
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", "\"", "&#39;", "'", "&nbsp;", " ")
	return r.Replace(s)
}

// ── get_current_time ─────────────────────────────────────────────────────────

func toolGetCurrentTime(argsJSON string) (string, error) {
	var args struct {
		Format string `json:"format"`
	}
	json.Unmarshal([]byte(argsJSON), &args)

	now := time.Now()
	layout := time.RFC3339
	if args.Format != "" && args.Format != "RFC3339" {
		layout = args.Format
	}

	result := map[string]string{
		"datetime":    now.Format(layout),
		"unix":        strconv.FormatInt(now.Unix(), 10),
		"timezone":    now.Location().String(),
		"day_of_week": now.Weekday().String(),
	}
	b, _ := json.Marshal(result)
	return string(b), nil
}

// ── calculator ───────────────────────────────────────────────────────────────

func toolCalculator(argsJSON string) (string, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Expression == "" {
		return "", fmt.Errorf("expression is required")
	}

	result, err := evalMathExpr(args.Expression)
	if err != nil {
		return "", fmt.Errorf("evaluation error: %w", err)
	}

	out := map[string]interface{}{
		"expression": args.Expression,
		"result":     result,
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// evalMathExpr evaluates simple math expressions safely using Go's AST parser.
// It supports: +, -, *, /, parentheses, and function calls sqrt/pow/abs/sin/cos/tan/log/exp/floor/ceil.
func evalMathExpr(expr string) (float64, error) {
	// Preprocess: replace common math function names with Go-parseable constructs
	// We'll parse as a Go expression and evaluate the AST
	node, err := parser.ParseExpr(expr)
	if err != nil {
		return 0, fmt.Errorf("cannot parse expression %q: %w", expr, err)
	}
	return evalNode(node)
}

func evalNode(node ast.Expr) (float64, error) {
	switch n := node.(type) {
	case *ast.BasicLit:
		if n.Kind == token.INT || n.Kind == token.FLOAT {
			return strconv.ParseFloat(n.Value, 64)
		}
		return 0, fmt.Errorf("unsupported literal: %s", n.Value)

	case *ast.UnaryExpr:
		val, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		if n.Op == token.SUB {
			return -val, nil
		}
		if n.Op == token.ADD {
			return val, nil
		}
		return 0, fmt.Errorf("unsupported unary op: %s", n.Op)

	case *ast.BinaryExpr:
		left, err := evalNode(n.X)
		if err != nil {
			return 0, err
		}
		right, err := evalNode(n.Y)
		if err != nil {
			return 0, err
		}
		switch n.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		case token.REM:
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			return math.Mod(left, right), nil
		default:
			return 0, fmt.Errorf("unsupported binary op: %s", n.Op)
		}

	case *ast.ParenExpr:
		return evalNode(n.X)

	case *ast.CallExpr:
		// Function calls like sqrt(x), pow(x,y), etc.
		ident, ok := n.Fun.(*ast.Ident)
		if !ok {
			return 0, fmt.Errorf("unsupported function call")
		}
		fname := strings.ToLower(ident.Name)
		args := make([]float64, len(n.Args))
		for i, a := range n.Args {
			v, err := evalNode(a)
			if err != nil {
				return 0, err
			}
			args[i] = v
		}
		return evalFunc(fname, args)

	case *ast.Ident:
		// Constants
		switch strings.ToLower(n.Name) {
		case "pi":
			return math.Pi, nil
		case "e":
			return math.E, nil
		default:
			return 0, fmt.Errorf("unknown identifier: %s", n.Name)
		}

	default:
		return 0, fmt.Errorf("unsupported expression type: %T", node)
	}
}

func evalFunc(name string, args []float64) (float64, error) {
	switch name {
	case "sqrt":
		if len(args) != 1 {
			return 0, fmt.Errorf("sqrt requires 1 argument")
		}
		return math.Sqrt(args[0]), nil
	case "pow":
		if len(args) != 2 {
			return 0, fmt.Errorf("pow requires 2 arguments")
		}
		return math.Pow(args[0], args[1]), nil
	case "abs":
		if len(args) != 1 {
			return 0, fmt.Errorf("abs requires 1 argument")
		}
		return math.Abs(args[0]), nil
	case "sin":
		if len(args) != 1 {
			return 0, fmt.Errorf("sin requires 1 argument")
		}
		return math.Sin(args[0]), nil
	case "cos":
		if len(args) != 1 {
			return 0, fmt.Errorf("cos requires 1 argument")
		}
		return math.Cos(args[0]), nil
	case "tan":
		if len(args) != 1 {
			return 0, fmt.Errorf("tan requires 1 argument")
		}
		return math.Tan(args[0]), nil
	case "log":
		if len(args) != 1 {
			return 0, fmt.Errorf("log requires 1 argument")
		}
		return math.Log(args[0]), nil
	case "log10":
		if len(args) != 1 {
			return 0, fmt.Errorf("log10 requires 1 argument")
		}
		return math.Log10(args[0]), nil
	case "exp":
		if len(args) != 1 {
			return 0, fmt.Errorf("exp requires 1 argument")
		}
		return math.Exp(args[0]), nil
	case "floor":
		if len(args) != 1 {
			return 0, fmt.Errorf("floor requires 1 argument")
		}
		return math.Floor(args[0]), nil
	case "ceil":
		if len(args) != 1 {
			return 0, fmt.Errorf("ceil requires 1 argument")
		}
		return math.Ceil(args[0]), nil
	case "round":
		if len(args) != 1 {
			return 0, fmt.Errorf("round requires 1 argument")
		}
		return math.Round(args[0]), nil
	case "max":
		if len(args) != 2 {
			return 0, fmt.Errorf("max requires 2 arguments")
		}
		return math.Max(args[0], args[1]), nil
	case "min":
		if len(args) != 2 {
			return 0, fmt.Errorf("min requires 2 arguments")
		}
		return math.Min(args[0], args[1]), nil
	default:
		return 0, fmt.Errorf("unknown function: %s", name)
	}
}

// ── read_file ────────────────────────────────────────────────────────────────

func toolReadFile(argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	data, err := os.ReadFile(args.Path)
	if err != nil {
		return "", fmt.Errorf("cannot read file %q: %w", args.Path, err)
	}
	content := string(data)
	const maxLen = 32000
	truncated := false
	if len(content) > maxLen {
		content = content[:maxLen]
		truncated = true
	}
	out := map[string]interface{}{"path": args.Path, "content": content}
	if truncated {
		out["truncated"] = true
		out["note"] = fmt.Sprintf("file truncated to %d chars", maxLen)
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}

// ── write_file ───────────────────────────────────────────────────────────────

func toolWriteFile(argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Path == "" {
		return "", fmt.Errorf("path and content are required")
	}
	if err := os.MkdirAll(filepath.Dir(args.Path), 0755); err != nil {
		return "", fmt.Errorf("cannot create directory: %w", err)
	}
	if err := os.WriteFile(args.Path, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("cannot write file %q: %w", args.Path, err)
	}
	b, _ := json.Marshal(map[string]interface{}{
		"path":    args.Path,
		"written": len(args.Content),
		"status":  "ok",
	})
	return string(b), nil
}

// ── list_directory ───────────────────────────────────────────────────────────

func toolListDirectory(argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &args)
	dir := args.Path
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("cannot list directory %q: %w", dir, err)
	}
	type entry struct {
		Name string `json:"name"`
		Type string `json:"type"`
		Size int64  `json:"size,omitempty"`
	}
	var list []entry
	for _, e := range entries {
		t := "file"
		if e.IsDir() {
			t = "dir"
		}
		var sz int64
		if !e.IsDir() {
			if info, err := e.Info(); err == nil {
				sz = info.Size()
			}
		}
		list = append(list, entry{Name: e.Name(), Type: t, Size: sz})
	}
	if list == nil {
		list = []entry{}
	}
	b, _ := json.Marshal(map[string]interface{}{"path": dir, "entries": list})
	return string(b), nil
}

// ── web_search ───────────────────────────────────────────────────────────────

func toolWebSearch(argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	results, err := WebSearch(args.Query, 4)
	if err != nil {
		out := map[string]interface{}{
			"query":  args.Query,
			"status": "error",
			"error":  err.Error(),
		}
		b, _ := json.Marshal(out)
		return string(b), nil
	}
	// Trim snippets to keep the result compact in the model's context window.
	for i := range results {
		if len(results[i].Snippet) > 160 {
			results[i].Snippet = results[i].Snippet[:160] + "…"
		}
	}
	out := map[string]interface{}{
		"query":   args.Query,
		"status":  "ok",
		"results": results,
	}
	if len(results) == 0 {
		out["message"] = "No results found."
	}
	b, _ := json.Marshal(out)
	return string(b), nil
}
