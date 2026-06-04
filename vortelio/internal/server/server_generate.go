package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/history"
	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
)

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if req.Model == "" {
		jsonError(w, 400, "model is required")
		return
	}
	if req.Steps == 0 {
		req.Steps = 20
	}

	ref, err := hub.ParseModelRef(req.Model)
	if err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	store := hub.NewModelStore()
	model, err := store.Resolve(ref)
	if err != nil {
		jsonError(w, 404, fmt.Sprintf("model %q not found. Download it first with vortelio pull", req.Model))
		return
	}

	hw := getHardware()
	if req.ForceCPU {
		hw.Backend = runtime.BackendCPU
	}

	runner, err := runtime.NewRunner(model, hw)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}

	streamBool := req.Stream == nil || *req.Stream
	opts := &runtime.RunOptions{
		Prompt: req.Prompt, InputFile: req.InputFile, OutputFile: req.OutputFile,
		Steps: req.Steps, Stream: streamBool, ForceCPU: req.ForceCPU,
		ContextSize: req.ContextSize,
	}

	if model.Type == "llm" {
		handleGenerateLLM(w, r, model, hw, opts, req)
		return
	}

	if _, ok := map[string]string{"image": "png", "audio": "wav", "video": "mp4", "3d": "obj"}[model.Type]; ok {
		handleGenerateMedia(w, r, model, runner, opts, req)
		return
	}

	if err := runner.Run(opts); err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]interface{}{"model": req.Model, "status": "done"})
}

func handleGenerateLLM(w http.ResponseWriter, r *http.Request, model *hub.Model, hw *runtime.Hardware, opts *runtime.RunOptions, req GenerateRequest) {
	// Parse format field
	var formatStr string
	if req.Format != nil {
		var s string
		if json.Unmarshal(req.Format, &s) == nil {
			formatStr = s
		} else {
			formatStr = string(req.Format)
		}
	}

	llmOpts := ollamaOptions(req.Options)

	// Determine streaming mode: default true for /api/generate
	streaming := req.Stream == nil || *req.Stream

	// Build NDJSON emitter helpers
	modelID := req.Model
	writeNDJSON := func(obj map[string]interface{}) {
		line, _ := json.Marshal(obj)
		w.Write(line)
		w.Write([]byte("\n"))
	}
	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	if streaming {
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
	}

	// Build messages: restore context session if client provides Context tokens
	var messages []map[string]interface{}
	if len(req.Messages) > 0 {
		for _, m := range req.Messages {
			messages = append(messages, map[string]interface{}{
				"role": m.Role, "content": m.Content,
			})
		}
	} else if len(req.Context) > 0 {
		// Restore prior conversation from context session store
		if hist := restoreContext(req.Context); hist != nil {
			for _, m := range hist {
				messages = append(messages, map[string]interface{}{
					"role": m["role"], "content": m["content"],
				})
			}
		}
	}

	// Agentic mode implies tools are on.
	toolsOn := req.ToolsEnabled || req.Agentic != nil

	// Smart/auto mode never hard-errors: if the model can't do tools, silently
	// degrade to a plain chat so the beginner always gets an answer.
	if toolsOn && req.Agentic != nil && req.Agentic.Auto && !runtime.ModelSupportsTools(req.Model) {
		req.Agentic = nil
		toolsOn = req.ToolsEnabled
	}

	// Native tool-support gate: communicate clearly when the model can't do tools.
	if toolsOn && !runtime.ModelSupportsTools(req.Model) {
		msg := runtime.ToolSupportMessage(req.Model)
		if streaming {
			w.Header().Set("Content-Type", "application/x-ndjson")
		} else {
			w.Header().Set("Content-Type", "application/json")
		}
		writeNDJSON(map[string]interface{}{
			"model": modelID, "created_at": time.Now().UTC().Format(time.RFC3339Nano),
			"response": "", "done": true, "done_reason": "error",
			"error": msg, "tools_unsupported": true,
		})
		flush()
		return
	}

	// Apply enabled skills as system-prompt augmentation.
	systemPrompt := req.System
	if req.Agentic != nil && len(req.Agentic.Skills) > 0 {
		systemPrompt = applySkills(systemPrompt, req.Agentic.Skills)
	}
	// Smart/auto mode: nudge the model to use its tools on its own.
	if req.Agentic != nil && req.Agentic.Auto {
		systemPrompt = autoSystemPrompt(systemPrompt)
	}

	sopts := runtime.StreamOpts{
		Prompt:       req.Prompt,
		System:       systemPrompt,
		Messages:     messages,
		Images:       req.Images,
		Raw:          req.Raw,
		Think:        req.Think,
		ToolsEnabled: toolsOn,
		Options:      llmOpts,
		Format:       formatStr,
	}

	var thinkBuf strings.Builder
	if req.Think {
		sopts.ThinkEmit = func(token string) {
			thinkBuf.WriteString(token)
			if streaming {
				writeNDJSON(map[string]interface{}{
					"model": modelID, "created_at": time.Now().UTC().Format(time.RFC3339Nano),
					"thinking": token, "done": false,
				})
				flush()
			}
		}
	}

	var toolEmitter runtime.ToolEventEmitter
	if toolsOn {
		toolEmitter = func(eventType string, data interface{}) {
			evJSON, _ := json.Marshal(data)
			if streaming {
				writeNDJSON(map[string]interface{}{
					"model": modelID, "created_at": time.Now().UTC().Format(time.RFC3339Nano),
					"event": eventType, "data": json.RawMessage(evJSON), "done": false,
				})
				flush()
			}
		}
	}

	// Build the composite tool provider for agentic requests.
	if req.Agentic != nil {
		sopts.ToolProvider = buildAgenticProvider(req.Agentic, toolEmitter)
	}

	var llmRunner *runtime.LLMRunner

	if req.ContextSize > 0 {
		llmRunner = runtime.NewLLMRunnerForServer(model, hw)
		llmRunner.SetContextSize(req.ContextSize)
		if err := llmRunner.EnsureServer(); err != nil {
			jsonError(w, 500, err.Error())
			return
		}
		defer llmRunner.StopServer()
	} else {
		ka := parseKeepAlive(req.KeepAlive)
		var loadErr error
		llmRunner, loadErr = runtime.GlobalModelManager.GetOrLoad(model, hw, ka)
		if loadErr != nil {
			jsonError(w, 500, loadErr.Error())
			return
		}
	}

	if !streaming {
		// Non-streaming: collect full response then send single JSON object
		var response strings.Builder
		err := llmRunner.StreamWithOpts(sopts, func(token string) {
			response.WriteString(token)
		}, toolEmitter)
		// Save context session
		newCtx := saveContext(req.Context, req.Prompt, response.String(), messages)
		obj := map[string]interface{}{
			"model": modelID, "created_at": time.Now().UTC().Format(time.RFC3339Nano),
			"response": response.String(), "done": true, "done_reason": "stop",
			"context": newCtx,
		}
		if thinkBuf.Len() > 0 {
			obj["thinking"] = thinkBuf.String()
		}
		if err != nil {
			obj["done_reason"] = "error"
			obj["error"] = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		writeNDJSON(obj)
		if req.Prompt != "" && response.Len() > 0 && !req.Incognito {
			convID := fmt.Sprintf("%d", time.Now().UnixNano())
			history.Append(
				history.Entry{ID: convID, Model: req.Model, Role: "user", Content: req.Prompt},
				history.Entry{ID: convID, Model: req.Model, Role: "assistant", Content: response.String()},
			)
		}
		return
	}

	// Streaming: emit token-per-line NDJSON
	var response strings.Builder
	err := llmRunner.StreamWithOpts(sopts, func(token string) {
		response.WriteString(token)
		writeNDJSON(map[string]interface{}{
			"model": modelID, "created_at": time.Now().UTC().Format(time.RFC3339Nano),
			"response": token, "done": false,
		})
		flush()
	}, toolEmitter)

	newCtx := saveContext(req.Context, req.Prompt, response.String(), messages)
	doneObj := map[string]interface{}{
		"model": modelID, "created_at": time.Now().UTC().Format(time.RFC3339Nano),
		"response": "", "done": true, "done_reason": "stop",
		"context": newCtx,
	}
	if thinkBuf.Len() > 0 {
		doneObj["thinking"] = thinkBuf.String()
	}
	if err != nil {
		doneObj["done_reason"] = "error"
		doneObj["error"] = err.Error()
	}
	writeNDJSON(doneObj)
	flush()

	if req.Prompt != "" && response.Len() > 0 && !req.Incognito {
		convID := fmt.Sprintf("%d", time.Now().UnixNano())
		history.Append(
			history.Entry{ID: convID, Model: req.Model, Role: "user", Content: req.Prompt},
			history.Entry{ID: convID, Model: req.Model, Role: "assistant", Content: response.String()},
		)
	}
}

func handleGenerateMedia(w http.ResponseWriter, r *http.Request, model *hub.Model, runner runtime.Runner, opts *runtime.RunOptions, req GenerateRequest) {
	mediaExt := map[string]string{"image": "png", "audio": "wav", "video": "mp4", "3d": "obj"}
	mediaMime := map[string]string{"image": "image/png", "audio": "audio/wav", "video": "video/mp4", "3d": "model/obj"}
	ext := mediaExt[model.Type]

	if opts.OutputFile == "" {
		tmp, err := os.CreateTemp("", "vortelio-out-*."+ext)
		if err != nil {
			jsonError(w, 500, "cannot create temporary file: "+err.Error())
			return
		}
		tmp.Close()
		opts.OutputFile = tmp.Name()
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, canFlush := w.(http.Flusher)

	sseEvent := func(typ, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ, data)
		if canFlush {
			flusher.Flush()
		}
	}

	ctx := r.Context()
	progCh := make(chan runtime.ProgressEvent, 32)
	done := make(chan error, 1)

	go func() {
		switch model.Type {
		case "image":
			if ir, ok := runner.(*runtime.ImageRunner); ok {
				done <- ir.RunWithProgress(opts, progCh)
				return
			}
		case "audio":
			if ar, ok := runner.(*runtime.AudioRunner); ok {
				done <- ar.RunWithProgress(opts, progCh)
				return
			}
		case "video":
			if vr, ok := runner.(*runtime.VideoRunner); ok {
				done <- vr.RunWithProgress(opts, progCh)
				return
			}
		case "3d":
			if tr, ok := runner.(*runtime.ThreeDRunner); ok {
				done <- tr.RunWithProgress(opts, progCh)
				return
			}
		}
		done <- runner.Run(opts)
		close(progCh)
	}()

	for ev := range progCh {
		select {
		case <-ctx.Done():
			sseEvent("error", `{"error":"generation cancelled"}`)
			return
		default:
		}
		progJSON, _ := json.Marshal(map[string]interface{}{"pct": ev.Percent, "msg": ev.Message})
		sseEvent("progress", string(progJSON))
	}

	if runErr := <-done; runErr != nil {
		errJSON, _ := json.Marshal(map[string]string{"error": runErr.Error()})
		sseEvent("error", string(errJSON))
		return
	}

	time.Sleep(200 * time.Millisecond)
	data, err := os.ReadFile(opts.OutputFile)
	if err != nil {
		errJSON, _ := json.Marshal(map[string]string{"error": "cannot read output: " + err.Error()})
		sseEvent("error", string(errJSON))
		return
	}
	dlDir := runtime.DefaultOutputDir()
	dlPath := filepath.Join(dlDir, fmt.Sprintf("vortelio-%s.%s", model.Type, ext))
	os.WriteFile(dlPath, data, 0644)

	resultJSON, _ := json.Marshal(map[string]interface{}{
		"model": req.Model, "status": "done", "type": model.Type,
		"mime":     mediaMime[model.Type],
		"data":     base64.StdEncoding.EncodeToString(data),
		"saved_to": dlPath,
	})
	sseEvent("result", string(resultJSON))
}

// restoreContext looks up a prior conversation session by context token array.
func restoreContext(ctx []int) []map[string]string {
	key := contextToKey(ctx)
	if key == "" {
		return nil
	}
	contextSessions.mu.Lock()
	defer contextSessions.mu.Unlock()
	return contextSessions.data[key]
}

// saveContext stores the conversation turn and returns a new context token array.
func saveContext(prevCtx []int, prompt, response string, existingMsgs []map[string]interface{}) []int {
	if prompt == "" || response == "" {
		return nil
	}

	// Restore prior history
	var history []map[string]string
	if len(prevCtx) > 0 {
		history = restoreContext(prevCtx)
	}
	// Append from existing messages if any
	if len(existingMsgs) > 0 {
		history = nil
		for _, m := range existingMsgs {
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			history = append(history, map[string]string{"role": role, "content": content})
		}
	}
	history = append(history,
		map[string]string{"role": "user", "content": prompt},
		map[string]string{"role": "assistant", "content": response},
	)
	// Trim to last 20 messages to avoid unbounded growth
	if len(history) > 20 {
		history = history[len(history)-20:]
	}

	newCtx := contextFromKey(fmt.Sprintf("%d", time.Now().UnixNano()))
	key := contextToKey(newCtx)

	contextSessions.mu.Lock()
	contextSessions.data[key] = history
	contextSessions.mu.Unlock()

	return newCtx
}
