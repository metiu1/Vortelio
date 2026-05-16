package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
)

// ────────────────────────────────────────────────────────────────────────────
// /api/route — heuristic model router
// ────────────────────────────────────────────────────────────────────────────

func handleRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Task        string   `json:"task"`        // "chat" | "code" | "vision" | "embed" | "image" | "audio" | "video" | "3d"
		MinParams   string   `json:"min_params"`  // e.g. "7B"
		MaxParams   string   `json:"max_params"`  // e.g. "70B"
		Prompt      string   `json:"prompt"`      // optional, used for heuristic detect
		Capabilities []string `json:"capabilities"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}

	if req.Task == "" {
		req.Task = detectTaskFromPrompt(req.Prompt)
	}

	store := hub.NewModelStore()
	all, err := store.List()
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	wantType := taskToType(req.Task)
	var candidates []*hub.Model
	for _, m := range all {
		if m.Type != wantType {
			continue
		}
		if req.Task == "vision" {
			if m.MmProjPath == "" && !contains(m.Capabilities, "vision") {
				continue
			}
		}
		ok := true
		for _, c := range req.Capabilities {
			if !contains(m.Capabilities, c) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		candidates = append(candidates, m)
	}

	if len(candidates) == 0 {
		respond(w, 200, map[string]interface{}{"model": "", "reason": "no candidates for task=" + req.Task})
		return
	}

	// Score: prefer smaller for chat/embed, larger for code/reasoning
	pick := candidates[0]
	preferLarge := req.Task == "code" || req.Task == "reason" || req.Task == "vision"
	for _, m := range candidates[1:] {
		if preferLarge {
			if m.SizeBytes > pick.SizeBytes {
				pick = m
			}
		} else {
			if m.SizeBytes < pick.SizeBytes && m.SizeBytes > 0 {
				pick = m
			}
		}
	}

	respond(w, 200, map[string]interface{}{
		"model":      fmt.Sprintf("%s/%s:%s", pick.Type, pick.Name, pick.Tag),
		"task":       req.Task,
		"candidates": modelRefs(candidates),
		"size":       pick.SizeHuman(),
	})
}

func detectTaskFromPrompt(p string) string {
	if p == "" {
		return "chat"
	}
	lp := strings.ToLower(p)
	codeKw := []string{"function", "class", "def ", "import ", "```", "compile", "bug", "stack trace"}
	for _, k := range codeKw {
		if strings.Contains(lp, k) {
			return "code"
		}
	}
	if strings.Contains(lp, "summarize") || strings.Contains(lp, "summary") || strings.Contains(lp, "tl;dr") {
		return "chat"
	}
	return "chat"
}

func taskToType(task string) string {
	switch task {
	case "image":
		return "image"
	case "audio":
		return "audio"
	case "video":
		return "video"
	case "3d":
		return "3d"
	case "embed":
		return "llm" // embed models stored as llm
	default:
		return "llm"
	}
}

func modelRefs(ms []*hub.Model) []string {
	out := make([]string, len(ms))
	for i, m := range ms {
		out[i] = fmt.Sprintf("%s/%s:%s", m.Type, m.Name, m.Tag)
	}
	return out
}

func contains(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}

// ────────────────────────────────────────────────────────────────────────────
// /api/compare — fan-out to N models
// ────────────────────────────────────────────────────────────────────────────

func handleCompare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Models []string `json:"models"`
		Prompt string   `json:"prompt"`
		System string   `json:"system"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Models) == 0 || req.Prompt == "" {
		jsonError(w, 400, "models and prompt required")
		return
	}

	type result struct {
		Model    string  `json:"model"`
		Response string  `json:"response"`
		Error    string  `json:"error,omitempty"`
		DurMS    int64   `json:"duration_ms"`
	}

	results := make([]result, len(req.Models))
	var wg sync.WaitGroup
	hw := getHardware()

	for i, modelRef := range req.Models {
		wg.Add(1)
		go func(i int, modelRef string) {
			defer wg.Done()
			start := time.Now()
			results[i].Model = modelRef
			ref, err := hub.ParseModelRef(modelRef)
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			m, err := hub.NewModelStore().Resolve(ref)
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			runner, err := runtime.GlobalModelManager.GetOrLoad(m, hw, 5*time.Minute)
			if err != nil {
				results[i].Error = err.Error()
				return
			}
			var buf strings.Builder
			err = runner.StreamWithOpts(runtime.StreamOpts{
				Prompt: req.Prompt,
				System: req.System,
			}, func(tok string) { buf.WriteString(tok) }, nil)
			results[i].Response = buf.String()
			results[i].DurMS = time.Since(start).Milliseconds()
			if err != nil {
				results[i].Error = err.Error()
			}
		}(i, modelRef)
	}
	wg.Wait()

	respond(w, 200, map[string]interface{}{
		"prompt":  req.Prompt,
		"results": results,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// /api/structured — forced JSON output via format
// ────────────────────────────────────────────────────────────────────────────

func handleStructured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Model  string          `json:"model"`
		Prompt string          `json:"prompt"`
		System string          `json:"system"`
		Schema json.RawMessage `json:"schema"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if req.Model == "" || req.Prompt == "" {
		jsonError(w, 400, "model and prompt required")
		return
	}

	ref, err := hub.ParseModelRef(req.Model)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	m, err := hub.NewModelStore().Resolve(ref)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	runner, err := runtime.GlobalModelManager.GetOrLoad(m, getHardware(), 5*time.Minute)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	formatStr := "json"
	if len(req.Schema) > 0 {
		formatStr = string(req.Schema)
	}

	sys := req.System
	if sys == "" {
		sys = "You output only valid JSON. No prose, no markdown fences."
	}

	var buf strings.Builder
	err = runner.StreamWithOpts(runtime.StreamOpts{
		Prompt: req.Prompt,
		System: sys,
		Format: formatStr,
	}, func(tok string) { buf.WriteString(tok) }, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	raw := strings.TrimSpace(buf.String())
	// Strip code fences if model included them
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var parsed interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		respond(w, 200, map[string]interface{}{
			"model":     req.Model,
			"raw":       raw,
			"parsed":    nil,
			"parse_err": err.Error(),
		})
		return
	}
	respond(w, 200, map[string]interface{}{
		"model":  req.Model,
		"raw":    raw,
		"parsed": parsed,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// /api/summarize — map-reduce on long text
// ────────────────────────────────────────────────────────────────────────────

func handleSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Model     string `json:"model"`
		Text      string `json:"text"`
		ChunkSize int    `json:"chunk_size"`
		Style     string `json:"style"` // "bullets" | "paragraph" | "tldr"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if req.Model == "" || req.Text == "" {
		jsonError(w, 400, "model and text required")
		return
	}
	if req.ChunkSize == 0 {
		req.ChunkSize = 8000
	}

	ref, err := hub.ParseModelRef(req.Model)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	m, err := hub.NewModelStore().Resolve(ref)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	runner, err := runtime.GlobalModelManager.GetOrLoad(m, getHardware(), 5*time.Minute)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	chunks := chunkText(req.Text, req.ChunkSize)
	partials := make([]string, len(chunks))

	stylePrompt := "Summarize concisely."
	switch req.Style {
	case "bullets":
		stylePrompt = "Summarize as bullet points."
	case "tldr":
		stylePrompt = "Write a 2-sentence TL;DR."
	case "paragraph":
		stylePrompt = "Write a single coherent paragraph summary."
	}

	// Map phase
	for i, c := range chunks {
		var buf strings.Builder
		err := runner.StreamWithOpts(runtime.StreamOpts{
			Prompt: c,
			System: stylePrompt,
		}, func(tok string) { buf.WriteString(tok) }, nil)
		if err != nil {
			jsonError(w, 500, fmt.Sprintf("chunk %d failed: %s", i, err.Error()))
			return
		}
		partials[i] = buf.String()
	}

	// Reduce phase (skip if single chunk)
	final := partials[0]
	if len(partials) > 1 {
		combined := strings.Join(partials, "\n\n---\n\n")
		var buf strings.Builder
		err := runner.StreamWithOpts(runtime.StreamOpts{
			Prompt: combined,
			System: "Merge these partial summaries into one. " + stylePrompt,
		}, func(tok string) { buf.WriteString(tok) }, nil)
		if err != nil {
			jsonError(w, 500, "reduce failed: "+err.Error())
			return
		}
		final = buf.String()
	}

	respond(w, 200, map[string]interface{}{
		"model":    req.Model,
		"chunks":   len(chunks),
		"summary":  strings.TrimSpace(final),
		"partials": partials,
	})
}

func chunkText(text string, size int) []string {
	if len(text) <= size {
		return []string{text}
	}
	var out []string
	words := strings.Fields(text)
	var cur strings.Builder
	for _, w := range words {
		if cur.Len()+len(w)+1 > size {
			out = append(out, cur.String())
			cur.Reset()
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(w)
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// ────────────────────────────────────────────────────────────────────────────
// /api/think — forced chain-of-thought, returns reasoning + answer separately
// ────────────────────────────────────────────────────────────────────────────

func handleThink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		System string `json:"system"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if req.Model == "" || req.Prompt == "" {
		jsonError(w, 400, "model and prompt required")
		return
	}

	ref, err := hub.ParseModelRef(req.Model)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	m, err := hub.NewModelStore().Resolve(ref)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	runner, err := runtime.GlobalModelManager.GetOrLoad(m, getHardware(), 5*time.Minute)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	sys := req.System
	if sys == "" {
		sys = "You are a careful reasoner."
	}
	sys += " Wrap your full reasoning in <think>...</think> tags before giving the final answer."

	var thinkBuf, answerBuf strings.Builder
	err = runner.StreamWithOpts(runtime.StreamOpts{
		Prompt:    req.Prompt,
		System:    sys,
		Think:     true,
		ThinkEmit: func(tok string) { thinkBuf.WriteString(tok) },
	}, func(tok string) { answerBuf.WriteString(tok) }, nil)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	respond(w, 200, map[string]interface{}{
		"model":     req.Model,
		"thinking":  strings.TrimSpace(thinkBuf.String()),
		"answer":    strings.TrimSpace(answerBuf.String()),
	})
}

// ────────────────────────────────────────────────────────────────────────────
// /openapi.json — minimal spec advertising all endpoints
// ────────────────────────────────────────────────────────────────────────────

func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	spec := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]string{
			"title":       "Vortelio API",
			"description": "Local AI platform — Ollama-compatible + multimodal + agents",
			"version":     "1.0.0",
		},
		"servers": []map[string]string{{"url": "/"}},
		"paths": map[string]interface{}{
			"/api/status":           opSpec("GET", "Server status, version, hardware"),
			"/api/version":          opSpec("GET", "Server version (Ollama-compat)"),
			"/api/ps":               opSpec("GET", "List loaded models"),
			"/api/tags":             opSpec("GET", "List installed models"),
			"/api/models":           opSpec("GET", "List models with sizes"),
			"/api/show":             opSpec("POST", "Model details (modelfile, template, parameters, capabilities)"),
			"/api/pull":             opSpec("POST", "Download model (NDJSON stream)"),
			"/api/push":             opSpec("POST", "Push to registry (501 stub)"),
			"/api/copy":             opSpec("POST", "Copy/duplicate a model"),
			"/api/delete":           opSpec("DELETE", "Delete a model"),
			"/api/create":           opSpec("POST", "Create model from Modelfile"),
			"/api/quantize":         opSpec("POST", "Quantize a model"),
			"/api/blobs/{digest}":   opSpec("HEAD/POST", "Blob storage management"),
			"/api/generate":         opSpec("POST", "Generate text (NDJSON stream)"),
			"/api/chat":             opSpec("POST", "Chat with messages + tools"),
			"/api/embed":            opSpec("POST", "Generate embeddings (batch)"),
			"/api/embeddings":       opSpec("POST", "Legacy single-prompt embedding"),
			"/api/route":            opSpec("POST", "Heuristic model router"),
			"/api/compare":          opSpec("POST", "A/B compare N models on same prompt"),
			"/api/structured":       opSpec("POST", "JSON-schema-forced output"),
			"/api/summarize":        opSpec("POST", "Map-reduce summarization"),
			"/api/think":            opSpec("POST", "Forced chain-of-thought with separated reasoning"),
			"/api/gguf/inspect":     opSpec("POST", "Parse GGUF file metadata"),
			"/api/hooks":            opSpec("GET/POST/DELETE", "Webhook management"),
			"/api/audit":            opSpec("GET", "Recent audit log entries"),
			"/api/rag/ingest":       opSpec("POST", "Add documents to RAG store"),
			"/api/rag/query":        opSpec("POST", "Retrieve similar chunks"),
			"/api/import/ollama":    opSpec("POST", "Import models from local Ollama install"),
			"/api/models/mmproj":    opSpec("POST", "Set multimodal projector for a model"),
			"/api/agents/catalog":   opSpec("GET", "List installable AI agents"),
			"/api/agents/install":   opSpec("POST", "Install an agent"),
			"/api/agents/start":     opSpec("POST", "Start an agent"),
			"/api/agents/stop":      opSpec("POST", "Stop an agent"),
			"/v1/chat/completions":  opSpec("POST", "OpenAI-compatible chat"),
			"/v1/completions":       opSpec("POST", "OpenAI-compatible completions"),
			"/v1/embeddings":        opSpec("POST", "OpenAI-compatible embeddings"),
			"/v1/audio/transcriptions": opSpec("POST", "Whisper-compat STT"),
			"/v1/audio/speech":      opSpec("POST", "OpenAI-compat TTS"),
			"/v1/images/generations": opSpec("POST", "OpenAI-compat image generation"),
			"/metrics":              opSpec("GET", "Prometheus metrics"),
		},
	}
	respond(w, 200, spec)
}

func opSpec(method, desc string) map[string]interface{} {
	m := strings.ToLower(strings.Split(method, "/")[0])
	return map[string]interface{}{
		m: map[string]interface{}{
			"summary": desc,
		},
	}
}

// Avoid unused import warning when context unused in this build
var _ = context.Background
