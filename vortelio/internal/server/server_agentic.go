package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/mcp"
	rt "github.com/vortelio/vortelio/internal/runtime"
)

// ── Approval broker ────────────────────────────────────────────────────────────
//
// Risky coding tool calls (shell, write, edit) block on a decision delivered out
// of band by POST /api/agentic/approve. The streaming chat connection emits an
// "approval_request" tool event; the UI shows approve/deny and resolves it.

type approvalReq struct {
	ch        chan bool
	createdAt time.Time
}

var (
	approvalsMu sync.Mutex
	approvals   = map[string]*approvalReq{}
)

func registerApproval(id string) chan bool {
	ch := make(chan bool, 1)
	approvalsMu.Lock()
	approvals[id] = &approvalReq{ch: ch, createdAt: time.Now()}
	approvalsMu.Unlock()
	return ch
}

func resolveApproval(id string, ok bool) bool {
	approvalsMu.Lock()
	a, found := approvals[id]
	if found {
		delete(approvals, id)
	}
	approvalsMu.Unlock()
	if !found {
		return false
	}
	a.ch <- ok
	return true
}

// POST /api/agentic/approve  — {id, approved}
func handleAgenticApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "use POST")
		return
	}
	var req struct {
		ID       string `json:"id"`
		Approved bool   `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if !resolveApproval(req.ID, req.Approved) {
		jsonError(w, 404, "no pending approval with that id (it may have timed out)")
		return
	}
	respond(w, 200, map[string]string{"status": "ok"})
}

// autoSystemPrompt prepends a short instruction telling the model it can call
// the available tools (web search, math, files, media generation) on its own.
// Used in smart/auto mode so a beginner never has to toggle anything.
func autoSystemPrompt(existing string) string {
	nudge := "You are a friendly, helpful assistant. Chat naturally with the user and answer in their own " +
		"language. You also have tools available (web search and image/audio/video/3D generation) that you may " +
		"use when the user clearly needs up-to-date information or asks you to create media. For greetings, " +
		"casual conversation, or anything you already know, just reply normally without using a tool."
	if strings.TrimSpace(existing) == "" {
		return nudge
	}
	return nudge + "\n\n" + existing
}

// ── Agentic provider builder ───────────────────────────────────────────────────

// buildAgenticProvider assembles a composite tool provider from the request's
// AgenticConfig. emit is the per-request tool event emitter (used for approvals).
func buildAgenticProvider(cfg *AgenticConfig, emit rt.ToolEventEmitter) rt.ToolProvider {
	var providers []rt.ToolProvider

	if cfg.WebSearch || cfg.Builtins {
		providers = append(providers, &filteredBuiltins{web: cfg.WebSearch, rest: cfg.Builtins})
	}
	if cfg.MCP {
		providers = append(providers, mcp.Default().Provider())
	}
	if cfg.Coding {
		providers = append(providers, newCodingProvider(cfg, emit))
	}
	if cfg.Media {
		providers = append(providers, newMediaProvider(emit))
	}
	return rt.NewCompositeProvider(providers...)
}

// filteredBuiltins exposes the builtin tools, optionally limited to web_search.
type filteredBuiltins struct {
	web  bool
	rest bool
}

func (f *filteredBuiltins) Tools() []rt.ToolDef {
	var out []rt.ToolDef
	for _, t := range rt.BuiltinTools() {
		name := t.Function.Name
		if name == "web_search" {
			if f.web {
				out = append(out, t)
			}
			continue
		}
		if f.rest {
			out = append(out, t)
		}
	}
	return out
}

func (f *filteredBuiltins) Execute(name, args string) (string, error) {
	return rt.ExecuteTool(name, args)
}

// ── Coding tool provider ───────────────────────────────────────────────────────

type codingProvider struct {
	mode    string // "plan" | "ask" | "auto"
	root    string
	emit    rt.ToolEventEmitter
	counter int
	mu      sync.Mutex
}

func newCodingProvider(cfg *AgenticConfig, emit rt.ToolEventEmitter) *codingProvider {
	mode := cfg.Mode
	if mode == "" {
		mode = "ask"
	}
	root := cfg.WorkingDir
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}
	return &codingProvider{mode: mode, root: root, emit: emit}
}

func (c *codingProvider) Tools() []rt.ToolDef {
	defs := []rt.ToolDef{
		toolDef("read_file", "Read a UTF-8 text file from the workspace.",
			`{"type":"object","properties":{"path":{"type":"string","description":"File path, relative to the workspace root or absolute."}},"required":["path"]}`),
		toolDef("list_directory", "List files and folders at a path in the workspace.",
			`{"type":"object","properties":{"path":{"type":"string","description":"Directory path. Defaults to workspace root."}},"required":[]}`),
		toolDef("glob_search", "Find files matching a glob pattern (e.g. **/*.go).",
			`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern relative to workspace root."}},"required":["pattern"]}`),
		toolDef("grep_search", "Search file contents for a substring and return matching lines.",
			`{"type":"object","properties":{"query":{"type":"string"},"path":{"type":"string","description":"Optional sub-path to search. Defaults to workspace root."}},"required":["query"]}`),
		toolDef("write_file", "Create or overwrite a file with the given content. (requires approval)",
			`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		toolDef("edit_file", "Replace the first occurrence of old_text with new_text in a file. (requires approval)",
			`{"type":"object","properties":{"path":{"type":"string"},"old_text":{"type":"string"},"new_text":{"type":"string"}},"required":["path","old_text","new_text"]}`),
		toolDef("run_shell", "Run a shell command in the workspace and return its output. (requires approval)",
			`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`),
	}
	return defs
}

func toolDef(name, desc, schema string) rt.ToolDef {
	return rt.ToolDef{Type: "function", Function: rt.ToolFuncDef{
		Name: name, Description: desc, Parameters: json.RawMessage(schema),
	}}
}

func isRisky(name string) bool {
	switch name {
	case "write_file", "edit_file", "run_shell":
		return true
	}
	return false
}

func (c *codingProvider) Execute(name, argsJSON string) (string, error) {
	if isRisky(name) {
		switch c.mode {
		case "plan":
			return "", fmt.Errorf("blocked: in Plan mode the agent cannot modify files or run commands. Switch to Ask or Auto mode to apply changes")
		case "auto":
			// proceed without prompting
		default: // "ask"
			summary := riskSummary(name, argsJSON)
			if !c.requestApproval(name, summary, argsJSON) {
				return "", fmt.Errorf("denied by user")
			}
		}
	}

	switch name {
	case "read_file":
		return c.readFile(argsJSON)
	case "list_directory":
		return c.listDir(argsJSON)
	case "glob_search":
		return c.glob(argsJSON)
	case "grep_search":
		return c.grep(argsJSON)
	case "write_file":
		return c.writeFile(argsJSON)
	case "edit_file":
		return c.editFile(argsJSON)
	case "run_shell":
		return c.runShell(argsJSON)
	default:
		return "", fmt.Errorf("unknown coding tool: %s", name)
	}
}

func riskSummary(name, argsJSON string) string {
	var m map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &m)
	switch name {
	case "run_shell":
		return fmt.Sprintf("Run command: %v", m["command"])
	case "write_file":
		return fmt.Sprintf("Overwrite file: %v", m["path"])
	case "edit_file":
		return fmt.Sprintf("Edit file: %v", m["path"])
	}
	return name
}

// requestApproval emits an approval_request event and blocks until resolved.
func (c *codingProvider) requestApproval(tool, summary, argsJSON string) bool {
	c.mu.Lock()
	c.counter++
	id := fmt.Sprintf("appr_%d_%d", time.Now().UnixNano(), c.counter)
	c.mu.Unlock()

	ch := registerApproval(id)
	if c.emit != nil {
		c.emit("approval_request", map[string]interface{}{
			"id": id, "tool": tool, "summary": summary, "arguments": json.RawMessage(argsJSON),
		})
	}
	select {
	case ok := <-ch:
		return ok
	case <-time.After(5 * time.Minute):
		resolveApproval(id, false)
		return false
	}
}

// resolvePath maps a tool-supplied path into the workspace, enforcing containment
// when a root is configured.
func (c *codingProvider) resolvePath(p string) (string, error) {
	if p == "" {
		if c.root != "" {
			return c.root, nil
		}
		return ".", nil
	}
	var full string
	if filepath.IsAbs(p) {
		full = filepath.Clean(p)
	} else {
		base := c.root
		if base == "" {
			base = "."
		}
		full = filepath.Clean(filepath.Join(base, p))
	}
	if c.root != "" {
		rel, err := filepath.Rel(c.root, full)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path %q is outside the workspace root", p)
		}
	}
	return full, nil
}

func (c *codingProvider) readFile(argsJSON string) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if len(data) > 200*1024 {
		data = data[:200*1024]
	}
	return string(data), nil
}

func (c *codingProvider) listDir(argsJSON string) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", err
	}
	var items []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		items = append(items, name)
	}
	sort.Strings(items)
	b, _ := json.Marshal(map[string]interface{}{"path": full, "entries": items})
	return string(b), nil
}

func (c *codingProvider) glob(argsJSON string) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if a.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	base := c.root
	if base == "" {
		base = "."
	}
	var matches []string
	filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		rel = filepath.ToSlash(rel)
		if ok, _ := filepath.Match(a.Pattern, filepath.Base(path)); ok {
			matches = append(matches, rel)
		} else if matchGlobStar(a.Pattern, rel) {
			matches = append(matches, rel)
		}
		if len(matches) >= 500 {
			return filepath.SkipAll
		}
		return nil
	})
	b, _ := json.Marshal(map[string]interface{}{"matches": matches})
	return string(b), nil
}

// matchGlobStar handles simple ** patterns against a slash path.
func matchGlobStar(pattern, path string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}
	suffix := pattern[strings.LastIndex(pattern, "**")+2:]
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return true
	}
	ok, _ := filepath.Match(suffix, filepath.Base(path))
	return ok
}

func (c *codingProvider) grep(argsJSON string) (string, error) {
	var a struct {
		Query string `json:"query"`
		Path  string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if a.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	base, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	var hits []string
	filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		info, _ := d.Info()
		if info != nil && info.Size() > 2*1024*1024 {
			return nil
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, a.Query) {
				rel, _ := filepath.Rel(base, path)
				hits = append(hits, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), i+1, strings.TrimSpace(line)))
				if len(hits) >= 200 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	b, _ := json.Marshal(map[string]interface{}{"matches": hits})
	return string(b), nil
}

func (c *codingProvider) writeFile(argsJSON string) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(a.Content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), full), nil
}

func (c *codingProvider) editFile(argsJSON string) (string, error) {
	var a struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(data), a.OldText) {
		return "", fmt.Errorf("old_text not found in %s", a.Path)
	}
	updated := strings.Replace(string(data), a.OldText, a.NewText, 1)
	if err := os.WriteFile(full, []byte(updated), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", full), nil
}

func (c *codingProvider) runShell(argsJSON string) (string, error) {
	var a struct {
		Command string `json:"command"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Command) == "" {
		return "", fmt.Errorf("command is required")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", a.Command)
	} else {
		cmd = exec.Command("sh", "-c", a.Command)
	}
	if c.root != "" {
		cmd.Dir = c.root
	}
	out, err := cmd.CombinedOutput()
	res := string(out)
	if len(res) > 100*1024 {
		res = res[:100*1024] + "\n...[truncated]"
	}
	if err != nil {
		return res, fmt.Errorf("command exited with error: %v", err)
	}
	return res, nil
}
