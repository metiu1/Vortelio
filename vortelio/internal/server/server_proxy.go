package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/config"
	fb "github.com/vortelio/vortelio/internal/firebase"
)

// ── Premium model catalog ─────────────────────────────────────────────────────

type ProxyModel struct {
	ID          string // OpenRouter model ID
	DisplayName string
	MinPlan     string // "pro" or "enterprise"
	Type        string // "llm", "image"
}

var ProxyModels = []ProxyModel{
	// ── LLM — Pro ────────────────────────────────────────────────
	{ID: "anthropic/claude-sonnet-4-5",        DisplayName: "Claude Sonnet 4.5",     MinPlan: "pro",        Type: "llm"},
	{ID: "anthropic/claude-3-5-haiku-20241022", DisplayName: "Claude Haiku 3.5",     MinPlan: "pro",        Type: "llm"},
	{ID: "openai/gpt-4o-mini",                 DisplayName: "GPT-4o Mini",           MinPlan: "pro",        Type: "llm"},
	{ID: "openai/gpt-4o",                      DisplayName: "GPT-4o",                MinPlan: "pro",        Type: "llm"},
	{ID: "google/gemini-2.0-flash-001",        DisplayName: "Gemini 2.0 Flash",      MinPlan: "pro",        Type: "llm"},
	{ID: "meta-llama/llama-3.3-70b-instruct",  DisplayName: "Llama 3.3 70B",         MinPlan: "pro",        Type: "llm"},
	{ID: "mistralai/mistral-large-2411",       DisplayName: "Mistral Large",         MinPlan: "pro",        Type: "llm"},
	{ID: "deepseek/deepseek-chat",             DisplayName: "DeepSeek Chat",         MinPlan: "pro",        Type: "llm"},
	// ── LLM — Enterprise ─────────────────────────────────────────
	{ID: "anthropic/claude-opus-4-5",          DisplayName: "Claude Opus 4.5",       MinPlan: "enterprise", Type: "llm"},
	{ID: "openai/o3",                          DisplayName: "OpenAI o3",             MinPlan: "enterprise", Type: "llm"},
	{ID: "openai/o4-mini",                     DisplayName: "OpenAI o4-mini",        MinPlan: "enterprise", Type: "llm"},
	{ID: "google/gemini-2.5-pro",              DisplayName: "Gemini 2.5 Pro",        MinPlan: "enterprise", Type: "llm"},
	{ID: "x-ai/grok-3",                        DisplayName: "Grok 3",               MinPlan: "enterprise", Type: "llm"},
}

// planRank maps plan names to a numeric rank for comparison.
var planRank = map[string]int{"free": 0, "pro": 1, "enterprise": 2}

func planAllowed(userPlan, minPlan string) bool {
	return planRank[userPlan] >= planRank[minPlan]
}

func findProxyModel(modelID string) (ProxyModel, bool) {
	for _, m := range ProxyModels {
		if m.ID == modelID {
			return m, true
		}
	}
	return ProxyModel{}, false
}

// ── GET /api/proxy/models ─────────────────────────────────────────────────────
// Returns the list of available premium models for the caller's plan.

func handleProxyModels(w http.ResponseWriter, r *http.Request) {
	uid := uidFromRequest(r)
	plan := "free"
	if uid != "" {
		if p, err := fb.GetUserProfile(r.Context(), uid); err == nil {
			plan = p.Plan
		}
	}
	type modelOut struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
		MinPlan     string `json:"min_plan"`
		Type        string `json:"type"`
		Available   bool   `json:"available"`
	}
	out := make([]modelOut, len(ProxyModels))
	for i, m := range ProxyModels {
		out[i] = modelOut{
			ID:          m.ID,
			DisplayName: m.DisplayName,
			MinPlan:     m.MinPlan,
			Type:        m.Type,
			Available:   planAllowed(plan, m.MinPlan),
		}
	}
	respond(w, 200, map[string]interface{}{"models": out, "plan": plan})
}

// ── POST /api/proxy/chat ──────────────────────────────────────────────────────
// OpenAI-compatible chat completions proxied through OpenRouter.
// Requires: valid user API key (vt_live_...) OR Firebase token, plan >= model's MinPlan.

func handleProxyChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}

	masterKey := config.Get().OpenRouterKey
	if masterKey == "" {
		respond(w, 503, map[string]string{"error": "cloud proxy not configured"})
		return
	}

	// Auth — accept both user API key and Firebase token
	uid, plan, err := resolveCallerPlan(r)
	if err != nil || uid == "" {
		respond(w, 401, map[string]string{"error": "authentication required"})
		return
	}

	// Parse request body (cap at 512KB)
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respond(w, 400, map[string]string{"error": "invalid request body"})
		return
	}

	var req struct {
		Model  string `json:"model"`
		Stream *bool  `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.Model == "" {
		respond(w, 400, map[string]string{"error": "model field required"})
		return
	}

	// Gate: check model is in catalog and plan is sufficient
	m, ok := findProxyModel(req.Model)
	if !ok {
		respond(w, 400, map[string]string{"error": fmt.Sprintf("model %q not available — check /api/proxy/models", req.Model)})
		return
	}
	if !planAllowed(plan, m.MinPlan) {
		respond(w, 403, map[string]string{
			"error": fmt.Sprintf("model %q requires %s plan (current: %s)", req.Model, m.MinPlan, plan),
		})
		return
	}

	// Gate: check monthly token budget
	if err := fb.CheckLimit(r.Context(), uid, plan); err != nil {
		respond(w, 429, map[string]string{"error": err.Error()})
		return
	}

	// Forward to OpenRouter
	streaming := req.Stream == nil || *req.Stream
	upstream, err := proxyToOpenRouter(r.Context(), masterKey, body, streaming)
	if err != nil {
		slog.Error("openrouter proxy error", "uid", uid, "model", req.Model, "err", err)
		respond(w, 502, map[string]string{"error": "upstream error"})
		return
	}
	defer upstream.Body.Close()

	// Stream response back
	w.Header().Set("Content-Type", upstream.Header.Get("Content-Type"))
	w.WriteHeader(upstream.StatusCode)

	var tokensIn, tokensOut int64
	if streaming && strings.Contains(upstream.Header.Get("Content-Type"), "text/event-stream") {
		tokensIn, tokensOut = streamSSEAndCount(w, upstream.Body)
	} else {
		respBody, _ := io.ReadAll(upstream.Body)
		w.Write(respBody)
		tokensIn, tokensOut = extractTokenCounts(respBody)
	}

	// Record usage async — don't block the response
	go func() {
		if err := fb.RecordUsage(r.Context(), uid, tokensIn, tokensOut); err != nil {
			slog.Error("usage record failed", "uid", uid, "err", err)
		}
	}()
}

// ── GET /api/proxy/usage ──────────────────────────────────────────────────────

func handleProxyUsage(w http.ResponseWriter, r *http.Request) {
	uid := uidFromRequest(r)
	if uid == "" {
		respond(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	usage, err := fb.GetUsage(r.Context(), uid)
	if err != nil {
		respond(w, 500, map[string]string{"error": "internal error"})
		return
	}
	profile, _ := fb.GetUserProfile(r.Context(), uid)
	plan := "free"
	if profile != nil {
		plan = profile.Plan
	}
	limit := fb.PlanLimits[plan]
	respond(w, 200, map[string]interface{}{
		"usage": usage,
		"plan":  plan,
		"limit": limit,
		"month": usage.Month,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveCallerPlan returns the UID and plan for the caller,
// checking user API key first (Authorization: Bearer vt_live_...) then Firebase token.
func resolveCallerPlan(r *http.Request) (uid, plan string, err error) {
	if !fb.Enabled() {
		return "", "", fmt.Errorf("firebase not configured")
	}

	auth := r.Header.Get("Authorization")
	rawKey := ""
	if strings.HasPrefix(auth, "Bearer vt_live_") {
		rawKey = strings.TrimPrefix(auth, "Bearer ")
	}

	if rawKey != "" {
		// User API key path
		foundUID, profile, err := fb.LookupAPIKey(r.Context(), rawKey)
		if err != nil {
			return "", "", err
		}
		if foundUID == "" || profile == nil {
			return "", "", fmt.Errorf("invalid api key")
		}
		return foundUID, profile.Plan, nil
	}

	// Firebase token path
	uid = uidFromRequest(r)
	if uid == "" {
		return "", "", fmt.Errorf("unauthorized")
	}
	profile, err := fb.GetUserProfile(r.Context(), uid)
	if err != nil || profile == nil {
		return uid, "free", nil
	}
	return uid, profile.Plan, nil
}

func proxyToOpenRouter(ctx interface{ Done() <-chan struct{} }, masterKey string, body []byte, streaming bool) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+masterKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://vortelio.app")
	req.Header.Set("X-Title", "Vortelio")

	client := &http.Client{Timeout: time.Duration(config.Get().CloudTimeoutSec) * time.Second}
	return client.Do(req)
}

func streamSSEAndCount(w http.ResponseWriter, body io.Reader) (tokensIn, tokensOut int64) {
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
			// Rough token count from SSE chunks (exact via usage chunk)
			chunk := buf[:n]
			if idx := bytes.Index(chunk, []byte(`"usage":`)); idx >= 0 {
				// Try to parse usage from the final SSE data chunk
				var wrapper struct {
					Usage struct {
						PromptTokens     int64 `json:"prompt_tokens"`
						CompletionTokens int64 `json:"completion_tokens"`
					} `json:"usage"`
				}
				if start := bytes.Index(chunk[idx:], []byte("{")); start >= 0 {
					json.Unmarshal(chunk[idx+start:], &wrapper)
					if wrapper.Usage.PromptTokens > 0 {
						tokensIn = wrapper.Usage.PromptTokens
						tokensOut = wrapper.Usage.CompletionTokens
					}
				}
			}
		}
		if err != nil {
			break
		}
	}
	return
}

func extractTokenCounts(body []byte) (tokensIn, tokensOut int64) {
	var resp struct {
		Usage struct {
			PromptTokens     int64 `json:"prompt_tokens"`
			CompletionTokens int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(body, &resp)
	return resp.Usage.PromptTokens, resp.Usage.CompletionTokens
}
