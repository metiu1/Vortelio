package cloud

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

// truncToolResult caps a tool result fed back to a cloud model so big outputs
// (e.g. list_directory of a large repo) don't bloat the request and trigger
// provider 500s. The UI still receives the full result via the tool event.
func truncToolResult(s string) string {
	const max = 4000
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n…[risultato troncato]"
}

// ── Provider catalog ──────────────────────────────────────────────────────────

type APIFormat string

const (
	FormatOpenAI    APIFormat = "openai"
	FormatAnthropic APIFormat = "anthropic"
	FormatGemini    APIFormat = "gemini"
)

type Provider struct {
	ID           string
	Name         string
	DefaultModel string
	BaseURL      string
	AuthHeader   string
	AuthPrefix   string
	Format       APIFormat
	KeyHint      string
}

var Providers = []Provider{
	{
		ID:           "openai",
		Name:         "OpenAI",
		DefaultModel: "gpt-4o-mini",
		BaseURL:      "https://api.openai.com/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://platform.openai.com/api-keys",
	},
	{
		ID:           "anthropic",
		Name:         "Anthropic",
		DefaultModel: "claude-3-5-haiku-20241022",
		BaseURL:      "https://api.anthropic.com/v1/messages",
		AuthHeader:   "x-api-key",
		AuthPrefix:   "",
		Format:       FormatAnthropic,
		KeyHint:      "https://console.anthropic.com/keys",
	},
	{
		ID:           "gemini",
		Name:         "Google Gemini",
		DefaultModel: "gemini-2.0-flash",
		BaseURL:      "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		Format:       FormatGemini,
		KeyHint:      "https://aistudio.google.com/app/apikey",
	},
	{
		ID:           "groq",
		Name:         "Groq",
		DefaultModel: "llama3-8b-8192",
		BaseURL:      "https://api.groq.com/openai/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://console.groq.com/keys",
	},
	{
		ID:           "mistral",
		Name:         "Mistral AI",
		DefaultModel: "mistral-small-latest",
		BaseURL:      "https://api.mistral.ai/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://console.mistral.ai/api-keys",
	},
	{
		ID:           "openrouter",
		Name:         "OpenRouter",
		DefaultModel: "meta-llama/llama-3.1-8b-instruct:free",
		BaseURL:      "https://openrouter.ai/api/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://openrouter.ai/keys",
	},
	{
		ID:           "xai",
		Name:         "xAI (Grok)",
		DefaultModel: "grok-2-latest",
		BaseURL:      "https://api.x.ai/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://console.x.ai",
	},
	{
		ID:           "together",
		Name:         "Together AI",
		DefaultModel: "meta-llama/Llama-3.3-70B-Instruct-Turbo",
		BaseURL:      "https://api.together.xyz/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://api.together.xyz/settings/api-keys",
	},
	{
		ID:           "deepseek",
		Name:         "DeepSeek",
		DefaultModel: "deepseek-chat",
		BaseURL:      "https://api.deepseek.com/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://platform.deepseek.com/api_keys",
	},
	{
		ID:           "perplexity",
		Name:         "Perplexity",
		DefaultModel: "sonar",
		BaseURL:      "https://api.perplexity.ai/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://www.perplexity.ai/settings/api",
	},
	{
		ID:           "ollamacloud",
		Name:         "Ollama Cloud",
		DefaultModel: "gpt-oss:120b",
		BaseURL:      "https://ollama.com/v1/chat/completions",
		AuthHeader:   "Authorization",
		AuthPrefix:   "Bearer ",
		Format:       FormatOpenAI,
		KeyHint:      "https://ollama.com/settings/keys",
	},
}

func FindProvider(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// ── Key storage ───────────────────────────────────────────────────────────────

func keysPath() string {
	return filepath.Join(config.HomeDir(), "cloud_keys.json")
}

// LoadKey legge e decifra la API key. Supporta migrazione trasparente da formato plaintext.
func LoadKey(providerID string) string {
	data, err := os.ReadFile(keysPath())
	if err != nil {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	stored := m[providerID]
	if stored == "" {
		return ""
	}
	// Prova a decifrare (formato cifrato = base64 di dati DPAPI).
	raw, err := base64.StdEncoding.DecodeString(stored)
	if err != nil {
		// Non è base64 → vecchio formato plaintext → usalo direttamente.
		return stored
	}
	plain, err := decryptKey(raw)
	if err != nil {
		// Decifratura fallita → potrebbe essere base64 di testo normale → prova.
		return string(raw)
	}
	return string(plain)
}

// SaveKey cifra la API key con DPAPI e la salva in base64.
func SaveKey(providerID, key string) error {
	path := keysPath()
	os.MkdirAll(filepath.Dir(path), 0700)

	var m map[string]string
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &m)
	}
	if m == nil {
		m = map[string]string{}
	}

	encrypted, err := encryptKey([]byte(key))
	if err != nil {
		// Fallback: salva plaintext se la cifratura non è disponibile.
		m[providerID] = key
	} else {
		m[providerID] = base64.StdEncoding.EncodeToString(encrypted)
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// HasKey reports whether a stored API key exists for the provider.
func HasKey(providerID string) bool {
	return LoadKey(providerID) != ""
}

// DeleteKey removes the stored API key for the provider.
func DeleteKey(providerID string) error {
	path := keysPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil // nothing stored
	}
	var m map[string]string
	if json.Unmarshal(data, &m) != nil || m == nil {
		return nil
	}
	delete(m, providerID)
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0600)
}

// ── Streaming chat ────────────────────────────────────────────────────────────

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolCallOptions configures tool use for ChatWithTools.
type ToolCallOptions struct {
	// Tools in OpenAI-compatible format [{type:"function", function:{name, description, parameters}}].
	Tools interface{}
	// ExecTool executes a named tool with JSON arguments, returns result string.
	ExecTool func(name, args string) (string, error)
	// OnEvent is called for tool_call / tool_result events (for UI streaming).
	OnEvent func(typ string, data interface{})
	// MaxRounds overrides the tool-loop cap (0 = default 5); raised for autonomous sessions.
	MaxRounds int
}

// Chat sends messages and streams tokens to onToken. Returns full response text.
func Chat(p Provider, apiKey string, messages []Message, onToken func(string)) (string, error) {
	switch p.Format {
	case FormatOpenAI:
		return chatOpenAI(p, apiKey, messages, onToken)
	case FormatAnthropic:
		return chatAnthropic(p, apiKey, messages, onToken)
	case FormatGemini:
		return chatGemini(p, apiKey, messages, onToken)
	default:
		return "", fmt.Errorf("unknown API format")
	}
}

// ChatWithTools is like Chat but supports a tool-calling loop.
// If opts is nil or opts.Tools is nil, falls back to Chat.
func ChatWithTools(p Provider, apiKey string, messages []Message, opts *ToolCallOptions, onToken func(string)) (string, error) {
	if opts == nil || opts.Tools == nil {
		return Chat(p, apiKey, messages, onToken)
	}
	switch p.Format {
	case FormatOpenAI:
		return chatOpenAIWithTools(p, apiKey, messages, opts, onToken)
	case FormatAnthropic:
		return chatAnthropicWithTools(p, apiKey, messages, opts, onToken)
	case FormatGemini:
		return chatGeminiWithTools(p, apiKey, messages, opts, onToken)
	default:
		return "", fmt.Errorf("unknown API format")
	}
}

// ── OpenAI-compatible (OpenAI, Groq, Mistral, OpenRouter) ─────────────────────

func chatOpenAI(p Provider, apiKey string, messages []Message, onToken func(string)) (string, error) {
	body := map[string]interface{}{
		"model":    p.DefaultModel,
		"messages": messages,
		"stream":   true,
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", p.BaseURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(p.AuthHeader, p.AuthPrefix+apiKey)
	if p.ID == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://vortelio.app")
		req.Header.Set("X-Title", "Vortelio")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 {
			tok := chunk.Choices[0].Delta.Content
			if tok != "" {
				sb.WriteString(tok)
				if onToken != nil {
					onToken(tok)
				}
			}
		}
	}
	return sb.String(), scanner.Err()
}

// ── Anthropic ─────────────────────────────────────────────────────────────────

func chatAnthropic(p Provider, apiKey string, messages []Message, onToken func(string)) (string, error) {
	// Convert messages: extract system message if present
	var system string
	var msgs []map[string]string
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
		} else {
			msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
		}
	}

	body := map[string]interface{}{
		"model":      p.DefaultModel,
		"messages":   msgs,
		"max_tokens": 4096,
		"stream":     true,
	}
	if system != "" {
		body["system"] = system
	}
	data, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", p.BaseURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		var chunk struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Type == "content_block_delta" && chunk.Delta.Type == "text_delta" {
			tok := chunk.Delta.Text
			if tok != "" {
				sb.WriteString(tok)
				if onToken != nil {
					onToken(tok)
				}
			}
		}
	}
	return sb.String(), scanner.Err()
}

// ── Gemini (non-streaming, uses generateContent) ──────────────────────────────

// ── OpenAI tool use ───────────────────────────────────────────────────────────

func chatOpenAIWithTools(p Provider, apiKey string, messages []Message, opts *ToolCallOptions, onToken func(string)) (string, error) {
	// Use []interface{} so we can append tool role messages with extra fields.
	msgs := make([]interface{}, len(messages))
	for i, m := range messages {
		msgs[i] = map[string]string{"role": m.Role, "content": m.Content}
	}

	var finalContent strings.Builder
	maxRounds := 5
	if opts != nil && opts.MaxRounds > 0 {
		maxRounds = opts.MaxRounds
	}

	for round := 0; round < maxRounds; round++ {
		body := map[string]interface{}{
			"model":       p.DefaultModel,
			"messages":    msgs,
			"stream":      true,
			"tools":       opts.Tools,
			"tool_choice": "auto",
		}
		data, _ := json.Marshal(body)

		req, err := http.NewRequest("POST", p.BaseURL, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(p.AuthHeader, p.AuthPrefix+apiKey)
		if p.ID == "openrouter" {
			req.Header.Set("HTTP-Referer", "https://vortelio.app")
			req.Header.Set("X-Title", "Vortelio")
		}

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("network error: %w", err)
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}

		type toolCallAccum struct {
			id        string
			name      string
			arguments strings.Builder
		}
		toolCallMap := map[int]*toolCallAccum{}
		var contentBuf strings.Builder
		finishReason := ""

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content   string `json:"content"`
						ToolCalls []struct {
							Index    int    `json:"index"`
							ID       string `json:"id"`
							Function struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							} `json:"function"`
						} `json:"tool_calls"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(payload), &chunk) != nil || len(chunk.Choices) == 0 {
				continue
			}
			choice := chunk.Choices[0]
			if choice.Delta.Content != "" {
				contentBuf.WriteString(choice.Delta.Content)
				finalContent.WriteString(choice.Delta.Content)
				if onToken != nil {
					onToken(choice.Delta.Content)
				}
			}
			for _, tc := range choice.Delta.ToolCalls {
				acc, ok := toolCallMap[tc.Index]
				if !ok {
					acc = &toolCallAccum{id: tc.ID, name: tc.Function.Name}
					toolCallMap[tc.Index] = acc
				}
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.name = tc.Function.Name
				}
				acc.arguments.WriteString(tc.Function.Arguments)
			}
			if choice.FinishReason != nil {
				finishReason = *choice.FinishReason
			}
		}
		resp.Body.Close()

		// No tool calls → final answer.
		if finishReason != "tool_calls" || len(toolCallMap) == 0 {
			return finalContent.String(), scanner.Err()
		}

		// Build assistant message with tool_calls.
		tcList := make([]map[string]interface{}, 0, len(toolCallMap))
		for i := 0; i < len(toolCallMap); i++ {
			acc, ok := toolCallMap[i]
			if !ok {
				continue
			}
			if acc.id == "" {
				acc.id = fmt.Sprintf("call_%d_%d", round, i)
			}
			tcList = append(tcList, map[string]interface{}{
				"id":   acc.id,
				"type": "function",
				"function": map[string]string{
					"name":      acc.name,
					"arguments": acc.arguments.String(),
				},
			})
		}
		asstMsg := map[string]interface{}{
			"role":       "assistant",
			"tool_calls": tcList,
		}
		if contentBuf.Len() > 0 {
			asstMsg["content"] = contentBuf.String()
		} else {
			asstMsg["content"] = nil
		}
		msgs = append(msgs, asstMsg)

		// Execute each tool and add results.
		for _, tc := range tcList {
			fn := tc["function"].(map[string]string)
			name := fn["name"]
			args := fn["arguments"]
			id := tc["id"].(string)

			if opts.OnEvent != nil {
				opts.OnEvent("tool_call", map[string]string{"id": id, "name": name, "arguments": args})
			}

			result := ""
			errStr := ""
			if opts.ExecTool != nil {
				r, err := opts.ExecTool(name, args)
				if err != nil {
					errStr = err.Error()
					result = fmt.Sprintf(`{"error":%q}`, err.Error())
				} else {
					result = r
				}
			} else {
				result = `{"error":"no tool executor configured"}`
				errStr = "no tool executor configured"
			}

			if opts.OnEvent != nil {
				opts.OnEvent("tool_result", map[string]interface{}{
					"call_id": id, "name": name, "result": result, "error": errStr,
				})
			}

			msgs = append(msgs, map[string]string{
				"role":         "tool",
				"content":      truncToolResult(result),
				"tool_call_id": id,
			})
		}
	}

	return finalContent.String(), nil
}

// ── Anthropic tool use ────────────────────────────────────────────────────────

// openaiToolsToAnthropic converts OpenAI-format tool defs to Anthropic format.
func openaiToolsToAnthropic(tools interface{}) []map[string]interface{} {
	data, _ := json.Marshal(tools)
	var raw []struct {
		Function struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	result := make([]map[string]interface{}, len(raw))
	for i, t := range raw {
		result[i] = map[string]interface{}{
			"name":         t.Function.Name,
			"description":  t.Function.Description,
			"input_schema": json.RawMessage(t.Function.Parameters),
		}
	}
	return result
}

func chatAnthropicWithTools(p Provider, apiKey string, messages []Message, opts *ToolCallOptions, onToken func(string)) (string, error) {
	anthTools := openaiToolsToAnthropic(opts.Tools)

	// Messages as []interface{} to support content-block arrays.
	type textBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	type toolUseBlock struct {
		Type  string          `json:"type"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	}
	type toolResultBlock struct {
		Type      string `json:"type"`
		ToolUseID string `json:"tool_use_id"`
		Content   string `json:"content"`
	}

	var system string
	msgs := make([]interface{}, 0, len(messages))
	for _, m := range messages {
		if m.Role == "system" {
			system = m.Content
			continue
		}
		msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
	}

	var finalContent strings.Builder
	maxRounds := 5
	if opts != nil && opts.MaxRounds > 0 {
		maxRounds = opts.MaxRounds
	}

	for round := 0; round < maxRounds; round++ {
		_ = round
		body := map[string]interface{}{
			"model":      p.DefaultModel,
			"messages":   msgs,
			"max_tokens": 4096,
			"stream":     true,
			"tools":      anthTools,
		}
		if system != "" {
			body["system"] = system
		}
		data, _ := json.Marshal(body)

		req, err := http.NewRequest("POST", p.BaseURL, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("network error: %w", err)
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}

		// Accumulate tool_use blocks by index.
		type toolAccum struct {
			id   string
			name string
			args strings.Builder
		}
		toolMap := map[int]*toolAccum{}
		var textBuf strings.Builder
		stopReason := ""
		currentBlockIndex := -1
		currentBlockType := ""

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")

			var ev struct {
				Type         string `json:"type"`
				Index        int    `json:"index"`
				ContentBlock struct {
					Type string `json:"type"`
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"content_block"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json"`
					StopReason  string `json:"stop_reason"`
				} `json:"delta"`
			}
			if json.Unmarshal([]byte(payload), &ev) != nil {
				continue
			}

			switch ev.Type {
			case "content_block_start":
				currentBlockIndex = ev.Index
				currentBlockType = ev.ContentBlock.Type
				if ev.ContentBlock.Type == "tool_use" {
					toolMap[ev.Index] = &toolAccum{id: ev.ContentBlock.ID, name: ev.ContentBlock.Name}
				}
			case "content_block_delta":
				if ev.Delta.Type == "text_delta" && currentBlockType == "text" {
					textBuf.WriteString(ev.Delta.Text)
					finalContent.WriteString(ev.Delta.Text)
					if onToken != nil {
						onToken(ev.Delta.Text)
					}
				} else if ev.Delta.Type == "input_json_delta" {
					if acc, ok := toolMap[currentBlockIndex]; ok {
						acc.args.WriteString(ev.Delta.PartialJSON)
					}
				}
			case "message_delta":
				stopReason = ev.Delta.StopReason
			}
		}
		resp.Body.Close()

		if stopReason != "tool_use" || len(toolMap) == 0 {
			return finalContent.String(), scanner.Err()
		}

		// Build assistant message with content blocks.
		var assistantContent []interface{}
		if textBuf.Len() > 0 {
			assistantContent = append(assistantContent, textBlock{Type: "text", Text: textBuf.String()})
		}
		for i := 0; i < len(toolMap); i++ {
			acc, ok := toolMap[i]
			if !ok {
				continue
			}
			argsRaw := json.RawMessage(acc.args.String())
			if len(argsRaw) == 0 {
				argsRaw = json.RawMessage(`{}`)
			}
			assistantContent = append(assistantContent, toolUseBlock{
				Type: "tool_use", ID: acc.id, Name: acc.name, Input: argsRaw,
			})
		}
		msgs = append(msgs, map[string]interface{}{"role": "assistant", "content": assistantContent})

		// Execute tools and build user tool_result message.
		var resultBlocks []interface{}
		for i := 0; i < len(toolMap); i++ {
			acc, ok := toolMap[i]
			if !ok {
				continue
			}
			argsStr := acc.args.String()

			if opts.OnEvent != nil {
				opts.OnEvent("tool_call", map[string]string{"id": acc.id, "name": acc.name, "arguments": argsStr})
			}

			result := ""
			errStr := ""
			if opts.ExecTool != nil {
				r, err := opts.ExecTool(acc.name, argsStr)
				if err != nil {
					errStr = err.Error()
					result = fmt.Sprintf(`{"error":%q}`, err.Error())
				} else {
					result = r
				}
			} else {
				result = `{"error":"no tool executor configured"}`
				errStr = "no tool executor configured"
			}

			if opts.OnEvent != nil {
				opts.OnEvent("tool_result", map[string]interface{}{
					"call_id": acc.id, "name": acc.name, "result": result, "error": errStr,
				})
			}

			resultBlocks = append(resultBlocks, toolResultBlock{
				Type: "tool_result", ToolUseID: acc.id, Content: truncToolResult(result),
			})
		}
		msgs = append(msgs, map[string]interface{}{"role": "user", "content": resultBlocks})
	}

	return finalContent.String(), nil
}

// ── Gemini tool use ───────────────────────────────────────────────────────────

// openaiToolsToGemini converts OpenAI-format tools to Gemini function_declarations.
func openaiToolsToGemini(tools interface{}) []map[string]interface{} {
	data, _ := json.Marshal(tools)
	var raw []struct {
		Function struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
		} `json:"function"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return nil
	}
	decls := make([]map[string]interface{}, len(raw))
	for i, t := range raw {
		decls[i] = map[string]interface{}{
			"name":        t.Function.Name,
			"description": t.Function.Description,
			"parameters":  json.RawMessage(t.Function.Parameters),
		}
	}
	return []map[string]interface{}{{"function_declarations": decls}}
}

func chatGeminiWithTools(p Provider, apiKey string, messages []Message, opts *ToolCallOptions, onToken func(string)) (string, error) {
	type part struct {
		Text         string                  `json:"text,omitempty"`
		FunctionCall *map[string]interface{} `json:"functionCall,omitempty"`
		FunctionResp *map[string]interface{} `json:"functionResponse,omitempty"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	// Gemini has a dedicated systemInstruction field. Collecting system messages
	// there (instead of injecting a synthetic "user" turn) keeps the contents
	// strictly alternating user/model — otherwise a system prompt followed by the
	// first real user message yields two consecutive user turns, which the API
	// rejects with 400.
	var systemTexts []string
	var contents []content
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			if strings.TrimSpace(m.Content) != "" {
				systemTexts = append(systemTexts, m.Content)
			}
			continue
		}
		contents = append(contents, content{Role: role, Parts: []part{{Text: m.Content}}})
	}

	geminiTools := openaiToolsToGemini(opts.Tools)
	var finalContent strings.Builder
	maxRounds := 5
	if opts != nil && opts.MaxRounds > 0 {
		maxRounds = opts.MaxRounds
	}
	url := p.BaseURL + "?key=" + apiKey
	client := &http.Client{Timeout: 120 * time.Second}

	for round := 0; round < maxRounds; round++ {
		_ = round
		body := map[string]interface{}{
			"contents": contents,
			"tools":    geminiTools,
		}
		if len(systemTexts) > 0 {
			body["systemInstruction"] = map[string]interface{}{
				"parts": []map[string]string{{"text": strings.Join(systemTexts, "\n\n")}},
			}
		}
		data, _ := json.Marshal(body)

		req, err := http.NewRequest("POST", url, bytes.NewReader(data))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("network error: %w", err)
		}
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return "", fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}

		var result struct {
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text         string `json:"text"`
						FunctionCall *struct {
							Name string                 `json:"name"`
							Args map[string]interface{} `json:"args"`
						} `json:"functionCall"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return "", fmt.Errorf("response parse error: %w", err)
		}
		resp.Body.Close()

		if len(result.Candidates) == 0 {
			return finalContent.String(), nil
		}
		cand := result.Candidates[0]

		// Collect text and function calls.
		var modelParts []part
		var funcCalls []struct {
			Name string
			Args map[string]interface{}
		}

		for _, p := range cand.Content.Parts {
			if p.Text != "" {
				finalContent.WriteString(p.Text)
				if onToken != nil {
					onToken(p.Text)
				}
				modelParts = append(modelParts, part{Text: p.Text})
			}
			if p.FunctionCall != nil {
				fc := map[string]interface{}{
					"name": p.FunctionCall.Name,
					"args": p.FunctionCall.Args,
				}
				modelParts = append(modelParts, part{FunctionCall: &fc})
				funcCalls = append(funcCalls, struct {
					Name string
					Args map[string]interface{}
				}{Name: p.FunctionCall.Name, Args: p.FunctionCall.Args})
			}
		}

		if len(funcCalls) == 0 {
			return finalContent.String(), nil
		}

		contents = append(contents, content{Role: "model", Parts: modelParts})

		// Execute tools and build function response parts.
		var respParts []part
		for _, fc := range funcCalls {
			argsJSON, _ := json.Marshal(fc.Args)
			argsStr := string(argsJSON)

			if opts.OnEvent != nil {
				opts.OnEvent("tool_call", map[string]string{"name": fc.Name, "arguments": argsStr})
			}

			result := ""
			errStr := ""
			if opts.ExecTool != nil {
				r, err := opts.ExecTool(fc.Name, argsStr)
				if err != nil {
					errStr = err.Error()
					result = fmt.Sprintf(`{"error":%q}`, err.Error())
				} else {
					result = r
				}
			} else {
				result = `{"error":"no tool executor configured"}`
				errStr = "no tool executor configured"
			}

			if opts.OnEvent != nil {
				opts.OnEvent("tool_result", map[string]interface{}{
					"name": fc.Name, "result": result, "error": errStr,
				})
			}

			result = truncToolResult(result)
			var responseObj interface{}
			if json.Unmarshal([]byte(result), &responseObj) != nil {
				responseObj = map[string]string{"output": result}
			}
			fr := map[string]interface{}{
				"name":     fc.Name,
				"response": responseObj,
			}
			respParts = append(respParts, part{FunctionResp: &fr})
		}
		contents = append(contents, content{Role: "user", Parts: respParts})
	}

	return finalContent.String(), nil
}

// ── Original Gemini (no tools) ────────────────────────────────────────────────

func chatGemini(p Provider, apiKey string, messages []Message, onToken func(string)) (string, error) {
	// Build Gemini content parts
	type part struct {
		Text string `json:"text"`
	}
	type content struct {
		Role  string `json:"role"`
		Parts []part `json:"parts"`
	}

	var systemTexts []string
	var contents []content
	for _, m := range messages {
		role := m.Role
		if role == "assistant" {
			role = "model"
		}
		if role == "system" {
			// Gemini has no system role in contents; collect into systemInstruction
			// so the conversation turns stay strictly alternating user/model.
			if strings.TrimSpace(m.Content) != "" {
				systemTexts = append(systemTexts, m.Content)
			}
			continue
		}
		contents = append(contents, content{Role: role, Parts: []part{{Text: m.Content}}})
	}

	body := map[string]interface{}{
		"contents": contents,
	}
	if len(systemTexts) > 0 {
		body["systemInstruction"] = map[string]interface{}{
			"parts": []map[string]string{{"text": strings.Join(systemTexts, "\n\n")}},
		}
	}
	data, _ := json.Marshal(body)

	url := p.BaseURL + "?key=" + apiKey
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("response parse error: %w", err)
	}

	var sb strings.Builder
	if len(result.Candidates) > 0 {
		for _, part := range result.Candidates[0].Content.Parts {
			sb.WriteString(part.Text)
		}
	}
	text := sb.String()
	if onToken != nil && text != "" {
		onToken(text)
	}
	return text, nil
}
