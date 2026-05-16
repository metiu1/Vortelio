package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
)

// ── Model CRUD ────────────────────────────────────────────────────────────────

func handleModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { jsonError(w, 405, "method not allowed"); return }
	store := hub.NewModelStore()
	models, err := store.List()
	if err != nil { jsonError(w, 500, err.Error()); return }
	out := make([]ModelWithSize, len(models))
	for i, m := range models { out[i] = ModelWithSize{Model: m, SizeHuman: m.SizeHuman()} }
	respond(w, 200, map[string]interface{}{"models": out})
}

func handleModelByName(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	for _, skip := range []string{"/api/models/remove", "/api/models/rename", "/api/models/info"} {
		if path == skip { http.NotFound(w, r); return }
	}
	rawPath := r.URL.RawPath
	if rawPath == "" { rawPath = r.URL.Path }
	raw := strings.TrimPrefix(rawPath, "/api/models/")
	raw = strings.ReplaceAll(raw, "%2F", "/")
	raw = strings.ReplaceAll(raw, "%2f", "/")
	raw = strings.ReplaceAll(raw, "%3A", ":")
	raw = strings.ReplaceAll(raw, "%3a", ":")
	raw = strings.ReplaceAll(raw, "%40", "@")
	if raw == "" { jsonError(w, 400, "missing model name"); return }
	ref, err := hub.ParseModelRef(raw)
	if err != nil { jsonError(w, 400, err.Error()); return }
	store := hub.NewModelStore()
	switch r.Method {
	case http.MethodGet:
		m, err := store.Resolve(ref)
		if err != nil { jsonError(w, 404, err.Error()); return }
		respond(w, 200, ModelWithSize{Model: m, SizeHuman: m.SizeHuman()})
	case http.MethodDelete:
		if err := store.Remove(ref); err != nil { jsonError(w, 500, err.Error()); return }
		respond(w, 200, map[string]string{"status": "deleted", "model": raw})
	case http.MethodPatch:
		var req struct{ DisplayName string `json:"display_name"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
		if err := store.Rename(ref, req.DisplayName); err != nil { jsonError(w, 500, err.Error()); return }
		respond(w, 200, map[string]string{"status": "renamed", "model": raw, "display_name": req.DisplayName})
	default:
		jsonError(w, 405, "use GET, DELETE or PATCH")
	}
}

func handleModelRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ Model string `json:"model"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	ref, err := hub.ParseModelRef(req.Model)
	if err != nil { jsonError(w, 400, err.Error()); return }
	store := hub.NewModelStore()
	if err := store.Remove(ref); err != nil { jsonError(w, 500, err.Error()); return }
	respond(w, 200, map[string]string{"status": "deleted", "model": req.Model})
}

func handleModelRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct {
		Model       string `json:"model"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	ref, err := hub.ParseModelRef(req.Model)
	if err != nil { jsonError(w, 400, err.Error()); return }
	store := hub.NewModelStore()
	if err := store.Rename(ref, req.DisplayName); err != nil { jsonError(w, 500, err.Error()); return }
	respond(w, 200, map[string]string{"status": "renamed", "model": req.Model, "display_name": req.DisplayName})
}

func handleModelInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ Model string `json:"model"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	ref, err := hub.ParseModelRef(req.Model)
	if err != nil { jsonError(w, 400, err.Error()); return }
	store := hub.NewModelStore()
	m, err := store.Resolve(ref)
	if err != nil { jsonError(w, 404, err.Error()); return }
	respond(w, 200, ModelWithSize{Model: m, SizeHuman: m.SizeHuman()})
}

func handleModelMmProj(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct {
		Model     string `json:"model"`
		MmProjPath string `json:"mmproj_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	ref, err := hub.ParseModelRef(req.Model)
	if err != nil { jsonError(w, 400, err.Error()); return }
	store := hub.NewModelStore()
	m, err := store.Resolve(ref)
	if err != nil { jsonError(w, 404, err.Error()); return }
	m.MmProjPath = req.MmProjPath
	if err := store.Save(m); err != nil { jsonError(w, 500, err.Error()); return }
	// Unload model so it restarts with new mmproj on next request
	runtime.GlobalModelManager.Unload(m)
	respond(w, 200, map[string]string{"status": "ok", "mmproj_path": req.MmProjPath})
}

// ── Download ──────────────────────────────────────────────────────────────────

func handlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct {
		Model    string `json:"model"`
		Name     string `json:"name"`     // Ollama legacy field
		Insecure bool   `json:"insecure"` // ignored
		Stream   *bool  `json:"stream"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	modelName := req.Model
	if modelName == "" { modelName = req.Name }
	if modelName == "" { jsonError(w, 400, "model is required"); return }
	ref, err := hub.ParseModelRef(modelName)
	if err != nil { jsonError(w, 400, err.Error()); return }

	// Detect Ollama-compat client via Accept header (NDJSON) vs browser GUI (SSE).
	wantSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")
	streaming := req.Stream == nil || *req.Stream

	if wantSSE {
		w.Header().Set("Content-Type", "text/event-stream")
	} else {
		w.Header().Set("Content-Type", "application/x-ndjson")
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, canFlush := w.(http.Flusher)

	emit := func(sseType string, obj map[string]interface{}) {
		data, _ := json.Marshal(obj)
		if wantSSE {
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", sseType, string(data))
		} else {
			fmt.Fprintf(w, "%s\n", string(data))
		}
		if canFlush { flusher.Flush() }
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	registerPull(modelName, cancel)
	defer unregisterPull(modelName)

	if streaming {
		emit("progress", map[string]interface{}{
			"status": "pulling manifest",
			"pct":    0,
			"msg":    "Connecting to HuggingFace...",
		})
	}

	doneCh := make(chan error, 1)
	go func() {
		d := hub.NewDownloaderWithContext(ctx)
		cb := func(downloaded, total int64) {
			if !streaming { return }
			var pct int; var msg string
			if total > 0 {
				pct = int(downloaded * 100 / total)
				if pct > 99 { pct = 99 }
				msg = fmt.Sprintf("%.1f / %.1f MB (%d%%)", float64(downloaded)/1e6, float64(total)/1e6, pct)
			} else {
				pct = 5
				msg = fmt.Sprintf("%.1f MB downloaded...", float64(downloaded)/1e6)
			}
			emit("progress", map[string]interface{}{
				"status":    "downloading",
				"completed": downloaded,
				"total":     total,
				"pct":       pct,
				"msg":       msg,
			})
		}
		doneCh <- d.Pull(ref, cb)
	}()

	select {
	case err := <-doneCh:
		if err != nil {
			emit("error", map[string]interface{}{"error": err.Error()})
		} else {
			emit("done", map[string]interface{}{"status": "success", "model": modelName})
		}
	case <-r.Context().Done():
		cancel()
	}
}

func handlePullCancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ Model string `json:"model"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	if cancelPull(req.Model) {
		respond(w, 200, map[string]string{"status": "cancelled", "model": req.Model})
	} else {
		jsonError(w, 404, "download not found or already completed")
	}
}
