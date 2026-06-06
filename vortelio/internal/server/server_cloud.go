package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/vortelio/vortelio/internal/cloud"
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
}

// GET /api/cloud/providers
// Lists providers, whether a key is stored, and the model choices.
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
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, 400, "invalid request")
		return
	}
	p, ok := cloud.FindProvider(req.Provider)
	if !ok {
		jsonError(w, 400, "unknown provider")
		return
	}
	key := cloud.LoadKey(req.Provider)
	if key == "" {
		jsonError(w, 400, fmt.Sprintf("no API key for %s — add your own key in Cloud Models", p.Name))
		return
	}
	if len(req.Messages) == 0 {
		jsonError(w, 400, "messages required")
		return
	}
	// Override the model if the caller picked one.
	if req.Model != "" {
		p.DefaultModel = req.Model
		if p.Format == cloud.FormatGemini {
			p.BaseURL = "https://generativelanguage.googleapis.com/v1beta/models/" + req.Model + ":generateContent"
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
