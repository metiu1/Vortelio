package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vortelio/vortelio/internal/cloud"
	rt "github.com/vortelio/vortelio/internal/runtime"
)

// ── BYOK (bring your own key) cloud models ────────────────────────────────────
//
// Anyone can use cloud models by entering their own provider API key — no
// subscription. Keys are stored locally (encrypted on Windows via DPAPI) by the
// cloud package; requests are made directly to the provider with the user's key
// and streamed back as a uniform SSE stream of {"delta": "..."} events.

// cloudModelChoices is the curated model list shown in the picker per provider.
// Users can still send any model string via the API; this is just for the UI.
var cloudModelChoices = map[string][][2]string{
	"openai": {
		{"gpt-4o", "GPT-4o"},
		{"gpt-4o-mini", "GPT-4o mini"},
		{"o3-mini", "o3-mini"},
		{"o1-mini", "o1-mini"},
	},
	"anthropic": {
		{"claude-3-5-sonnet-20241022", "Claude 3.5 Sonnet"},
		{"claude-3-5-haiku-20241022", "Claude 3.5 Haiku"},
		{"claude-3-opus-20240229", "Claude 3 Opus"},
	},
	"gemini": {
		{"gemini-2.0-flash", "Gemini 2.0 Flash"},
		{"gemini-2.5-flash", "Gemini 2.5 Flash"},
		{"gemini-2.5-pro", "Gemini 2.5 Pro"},
	},
	"groq": {
		{"llama-3.3-70b-versatile", "Llama 3.3 70B"},
		{"llama3-8b-8192", "Llama 3 8B"},
	},
	"mistral": {
		{"mistral-small-latest", "Mistral Small"},
		{"mistral-large-latest", "Mistral Large"},
	},
	"openrouter": {
		{"meta-llama/llama-3.1-8b-instruct:free", "Llama 3.1 8B (free)"},
		{"anthropic/claude-3.5-sonnet", "Claude 3.5 Sonnet"},
		{"openai/gpt-4o", "GPT-4o"},
		{"deepseek/deepseek-r1", "DeepSeek R1"},
	},
	"xai": {
		{"grok-2-latest", "Grok 2"},
		{"grok-2-vision-latest", "Grok 2 Vision"},
		{"grok-beta", "Grok Beta"},
	},
	"together": {
		{"meta-llama/Llama-3.3-70B-Instruct-Turbo", "Llama 3.3 70B Turbo"},
		{"Qwen/Qwen2.5-72B-Instruct-Turbo", "Qwen2.5 72B Turbo"},
		{"mistralai/Mixtral-8x7B-Instruct-v0.1", "Mixtral 8x7B"},
	},
	"deepseek": {
		{"deepseek-chat", "DeepSeek V3 (chat)"},
		{"deepseek-reasoner", "DeepSeek R1 (reasoner)"},
	},
	"perplexity": {
		{"sonar", "Sonar"},
		{"sonar-pro", "Sonar Pro"},
		{"sonar-reasoning", "Sonar Reasoning"},
	},
	"ollamacloud": {
		{"gpt-oss:120b", "gpt-oss 120B"},
		{"deepseek-v3.1:671b", "DeepSeek V3.1 671B"},
		{"qwen3-coder:480b", "Qwen3 Coder 480B"},
		{"kimi-k2:1t", "Kimi K2 1T"},
	},
}

// GET /api/cloud/providers
// Lists providers, whether a key is stored, and the model choices.
// CLICloudModel describes a ready-to-use cloud model for the CLI picker.
type CLICloudModel struct {
	Provider     string
	ProviderName string
	Model        string
	Label        string
}

// CloudModelsForCLI returns the cloud models the user can use (providers with a
// saved API key), for the `vortelio code` /model picker.
func CloudModelsForCLI() []CLICloudModel {
	var out []CLICloudModel
	for _, p := range cloud.Providers {
		if cloud.LoadKey(p.ID) == "" {
			continue
		}
		choices := cloudModelChoices[p.ID]
		if len(choices) == 0 {
			choices = [][2]string{{p.DefaultModel, p.DefaultModel}}
		}
		for _, c := range choices {
			out = append(out, CLICloudModel{Provider: p.ID, ProviderName: p.Name, Model: c[0], Label: c[1]})
		}
	}
	return out
}

// RunCLICloudTurn streams one cloud chat turn for the CLI, using the same agentic
// harness (tools) as the GUI. Returns the full assistant text.
func RunCLICloudTurn(providerID, model, workdir string, autonomous, mcpOn bool, skills []string, history []map[string]string, onToken func(string), emit rt.ToolEventEmitter) (string, error) {
	p, ok := cloud.FindProvider(providerID)
	if !ok {
		return "", fmt.Errorf("provider cloud sconosciuto: %s", providerID)
	}
	key := cloud.LoadKey(providerID)
	if key == "" {
		return "", fmt.Errorf("nessuna API key per %s", p.Name)
	}
	if model != "" {
		p.DefaultModel = model
		if p.Format == cloud.FormatGemini {
			p.BaseURL = "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent"
		}
	}
	prov, sys := BuildCLIHarness(workdir, "auto", autonomous, mcpOn, skills, emit)
	msgs := []cloud.Message{}
	if sys != "" {
		msgs = append(msgs, cloud.Message{Role: "system", Content: sys})
	}
	for _, m := range history {
		msgs = append(msgs, cloud.Message{Role: m["role"], Content: m["content"]})
	}
	toolOpts := &cloud.ToolCallOptions{Tools: prov.Tools(), ExecTool: prov.Execute, OnEvent: emit}
	if autonomous {
		toolOpts.MaxRounds = 40
	}
	return cloud.ChatWithTools(p, key, msgs, toolOpts, onToken)
}

// GET /api/media/providers — media (image/audio/video/3d) cloud services + key state.
func handleMediaProviders(w http.ResponseWriter, r *http.Request) {
	type out struct {
		ID           string `json:"id"`
		Name         string `json:"name"`
		Type         string `json:"type"`
		DefaultModel string `json:"default_model"`
		KeyHint      string `json:"key_hint"`
		HasKey       bool   `json:"has_key"`
	}
	res := make([]out, 0, len(cloud.MediaProviders))
	for _, p := range cloud.MediaProviders {
		res = append(res, out{p.ID, p.Name, p.Type, p.DefaultModel, p.KeyHint, cloud.LoadKey(p.ID) != ""})
	}
	respond(w, 200, map[string]interface{}{"providers": res})
}

// POST/DELETE /api/media/key — {provider, key} save or remove a media API key.
func handleMediaKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Key      string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		jsonError(w, 400, "invalid request")
		return
	}
	if _, ok := cloud.FindMediaProvider(req.Provider); !ok {
		jsonError(w, 400, "unknown media provider")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		cloud.DeleteKey(req.Provider)
		respond(w, 200, map[string]interface{}{"ok": true, "has_key": false})
	case http.MethodPost:
		if req.Key == "" {
			jsonError(w, 400, "key required")
			return
		}
		if err := cloud.SaveKey(req.Provider, req.Key); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		respond(w, 200, map[string]interface{}{"ok": true, "has_key": true})
	default:
		jsonError(w, 405, "use POST or DELETE")
	}
}

// normalizeChatURL turns a user-supplied server address into a full OpenAI-style
// chat-completions URL. Accepts "host:port", "http://host:port", ".../v1", or a
// full ".../chat/completions" URL.
func normalizeChatURL(u string) string {
	u = strings.TrimSpace(u)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		u = "http://" + u
	}
	if strings.Contains(u, "/chat/completions") {
		return u
	}
	u = strings.TrimRight(u, "/")
	if strings.HasSuffix(u, "/v1") {
		return u + "/chat/completions"
	}
	return u + "/v1/chat/completions"
}

func handleCloudProviders(w http.ResponseWriter, r *http.Request) {
	type modelOut struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}
	type providerOut struct {
		ID      string     `json:"id"`
		Name    string     `json:"name"`
		KeyHint string     `json:"key_hint"`
		HasKey  bool       `json:"has_key"`
		Models  []modelOut `json:"models"`
	}
	out := make([]providerOut, 0, len(cloud.Providers))
	for _, p := range cloud.Providers {
		models := []modelOut{}
		if choices, ok := cloudModelChoices[p.ID]; ok {
			for _, c := range choices {
				models = append(models, modelOut{ID: c[0], Label: c[1]})
			}
		} else {
			models = append(models, modelOut{ID: p.DefaultModel, Label: p.DefaultModel})
		}
		out = append(out, providerOut{
			ID:      p.ID,
			Name:    p.Name,
			KeyHint: p.KeyHint,
			HasKey:  cloud.HasKey(p.ID),
			Models:  models,
		})
	}
	respond(w, 200, map[string]interface{}{"providers": out})
}

// POST   /api/cloud/key   {"provider":"openai","key":"sk-..."}  → save
// DELETE /api/cloud/key   {"provider":"openai"}                 → remove
func handleCloudKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Key      string `json:"key"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		jsonError(w, 400, "invalid request")
		return
	}
	if req.Provider == "" {
		jsonError(w, 400, "provider required")
		return
	}
	if _, ok := cloud.FindProvider(req.Provider); !ok {
		jsonError(w, 400, "unknown provider")
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if err := cloud.DeleteKey(req.Provider); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		respond(w, 200, map[string]interface{}{"ok": true, "has_key": false})
	case http.MethodPost:
		if req.Key == "" {
			jsonError(w, 400, "key required")
			return
		}
		if err := cloud.SaveKey(req.Provider, req.Key); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		respond(w, 200, map[string]interface{}{"ok": true, "has_key": true})
	default:
		jsonError(w, 405, "method not allowed")
	}
}

// POST /api/cloud/chat
// Body: {"provider":"openai","model":"gpt-4o","messages":[{"role","content"}]}
// Streams SSE: data: {"delta":"..."}  then  data: [DONE]  (or data: {"error":"..."}).
func handleCloudChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "method not allowed")
		return
	}
	var req struct {
		Provider string          `json:"provider"`
		Model    string          `json:"model"`
		Messages []cloud.Message `json:"messages"`
		Agentic  *AgenticConfig  `json:"agentic"`
		System   string          `json:"system"`
		BaseURL  string          `json:"base_url"` // custom/self-hosted endpoint (vast.ai, home server, Ollama)
		Key      string          `json:"key"`      // optional key for the custom endpoint
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, 400, "invalid request")
		return
	}
	if len(req.Messages) == 0 {
		jsonError(w, 400, "messages required")
		return
	}

	var p cloud.Provider
	var key string
	if req.Provider == "custom" || req.BaseURL != "" {
		// Custom / self-hosted OpenAI- or Ollama-compatible endpoint.
		bu := normalizeChatURL(req.BaseURL)
		if bu == "" {
			jsonError(w, 400, "base_url required for a custom server")
			return
		}
		p = cloud.Provider{ID: "custom", Name: "Custom server", BaseURL: bu,
			AuthHeader: "Authorization", AuthPrefix: "Bearer ", Format: cloud.FormatOpenAI, DefaultModel: req.Model}
		key = req.Key // may be empty for no-auth home servers
	} else {
		var ok bool
		p, ok = cloud.FindProvider(req.Provider)
		if !ok {
			jsonError(w, 400, "unknown provider")
			return
		}
		key = cloud.LoadKey(req.Provider)
		if key == "" {
			jsonError(w, 400, fmt.Sprintf("no API key for %s — add your own key in Cloud Models", p.Name))
			return
		}
		// Override the model if the caller picked one.
		if req.Model != "" {
			p.DefaultModel = req.Model
			if p.Format == cloud.FormatGemini {
				p.BaseURL = "https://generativelanguage.googleapis.com/v1beta/models/" + req.Model + ":generateContent"
			}
		}
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, canFlush := w.(http.Flusher)
	emit := func(obj interface{}) {
		b, _ := json.Marshal(obj)
		fmt.Fprintf(w, "data: %s\n\n", b)
		if canFlush {
			flusher.Flush()
		}
	}

	// Apply enabled skills as a system-prompt augmentation, prepended as a
	// system message so cloud providers receive it.
	systemPrompt := req.System
	if req.Agentic != nil && len(req.Agentic.Skills) > 0 {
		systemPrompt = applySkills(systemPrompt, req.Agentic.Skills)
	}
	if req.Agentic != nil && req.Agentic.Auto {
		systemPrompt = autoSystemPrompt(systemPrompt)
	}
	if req.Agentic != nil && req.Agentic.Autonomous {
		systemPrompt = autonomousSystemPrompt(systemPrompt)
	}
	if ws := workspaceContext(req.Agentic); ws != "" {
		systemPrompt = ws + "\n\n" + systemPrompt
	}
	if systemPrompt != "" {
		req.Messages = append([]cloud.Message{{Role: "system", Content: systemPrompt}}, req.Messages...)
	}

	// Build agentic tool options when requested. The tool event emitter doubles
	// as the approval-request channel for risky coding tools.
	var toolOpts *cloud.ToolCallOptions
	if req.Agentic != nil {
		toolEmit := func(eventType string, data interface{}) {
			b, _ := json.Marshal(data)
			emit(map[string]interface{}{"event": eventType, "data": json.RawMessage(b)})
		}
		provider := buildAgenticProvider(req.Agentic, toolEmit)
		if tools := provider.Tools(); len(tools) > 0 {
			toolOpts = &cloud.ToolCallOptions{
				Tools:   tools,
				ExecTool: provider.Execute,
				OnEvent:  toolEmit,
			}
			if req.Agentic.Autonomous {
				toolOpts.MaxRounds = 40
			}
		}
	}

	_, err := cloud.ChatWithTools(p, key, req.Messages, toolOpts, func(tok string) {
		emit(map[string]string{"delta": tok})
	})
	if err != nil {
		emit(map[string]string{"error": err.Error()})
	}
	fmt.Fprint(w, "data: [DONE]\n\n")
	if canFlush {
		flusher.Flush()
	}
}
