package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/agent"
)

// ── Agent proxy ───────────────────────────────────────────────────────────────

func handleAgentCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct {
		URL    string `json:"url"`
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	if req.URL == "" { jsonError(w, 400, "url is required"); return }

	target := strings.TrimRight(req.URL, "/") + "/v1/models"
	hreq, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target, nil)
	if err != nil { jsonError(w, 400, "invalid url: "+err.Error()); return }
	if req.APIKey != "" { hreq.Header.Set("Authorization", "Bearer "+req.APIKey) }
	hreq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(hreq)
	if err != nil {
		respond(w, 200, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	respond(w, 200, map[string]interface{}{"ok": resp.StatusCode < 300, "status": resp.StatusCode, "body": string(body)})
}

func handleAgentProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct {
		URL          string                 `json:"url"`
		APIKey       string                 `json:"api_key"`
		Payload      map[string]interface{} `json:"payload"`
		Stream       bool                   `json:"stream"`
		Endpoint     string                 `json:"endpoint"`
		ExtraHeaders map[string]string      `json:"extra_headers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	if req.URL == "" { jsonError(w, 400, "url is required"); return }

	endpoint := "/v1/chat/completions"
	if req.Endpoint != "" { endpoint = req.Endpoint }
	target := strings.TrimRight(req.URL, "/") + endpoint

	if req.Payload == nil { req.Payload = map[string]interface{}{} }
	req.Payload["stream"] = req.Stream

	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil { jsonError(w, 400, "payload not serializable"); return }

	hreq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(payloadBytes))
	if err != nil { jsonError(w, 400, "invalid url: "+err.Error()); return }
	hreq.Header.Set("Content-Type", "application/json")

	if req.APIKey != "" {
		switch {
		case strings.Contains(req.URL, "anthropic.com"):
			hreq.Header.Set("x-api-key", req.APIKey)
		case strings.Contains(req.URL, "googleapis.com"):
			hreq.Header.Set("x-goog-api-key", req.APIKey)
		default:
			hreq.Header.Set("Authorization", "Bearer "+req.APIKey)
		}
	}
	for k, v := range req.ExtraHeaders { hreq.Header.Set(k, v) }
	if strings.Contains(req.URL, "anthropic.com") && hreq.Header.Get("anthropic-version") == "" {
		hreq.Header.Set("anthropic-version", "2023-06-01")
	}

	client := &http.Client{
		Timeout:   5 * time.Minute,
		Transport: &http.Transport{DisableCompression: true},
	}
	resp, err := client.Do(hreq)
	if err != nil { jsonError(w, 502, "cannot contact provider: "+err.Error()); return }
	defer resp.Body.Close()

	for k, v := range resp.Header {
		if strings.EqualFold(k, "content-type") || strings.EqualFold(k, "transfer-encoding") {
			w.Header()[k] = v
		}
	}
	if origin := r.Header.Get("Origin"); origin != "" && (strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1")) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 1)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if canFlush { flusher.Flush() }
		}
		if rerr != nil { break }
	}
}

// ── Agent manager ─────────────────────────────────────────────────────────────

func handleAgentCatalog(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(agent.CatalogJSON())
}

func handleAgentInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ ID string `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, canFlush := w.(http.Flusher)
	sseEvent := func(typ, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", typ, data)
		if canFlush { flusher.Flush() }
	}

	progJSON, _ := json.Marshal(map[string]interface{}{"msg": "Avvio npm install…"})
	sseEvent("progress", string(progJSON))

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	lineCount := 0
	err := agent.Install(ctx, req.ID, func(line string) {
		lineCount++
		pct := lineCount * 3
		if pct > 90 { pct = 90 }
		progJSON, _ := json.Marshal(map[string]interface{}{"pct": pct, "msg": line})
		sseEvent("progress", string(progJSON))
	})
	if err != nil {
		errMsg := strings.ReplaceAll(err.Error(), "\n", "\\n")
		errJSON, _ := json.Marshal(map[string]string{"error": errMsg})
		sseEvent("error", string(errJSON))
		return
	}
	sseEvent("done", fmt.Sprintf(`{"id":%q}`, req.ID))
}

func handleAgentStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ ID string `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	if err := agent.Start(req.ID); err != nil { jsonError(w, 500, err.Error()); return }

	// Poll health endpoint for up to 15s instead of fixed sleep
	deadline := time.Now().Add(15 * time.Second)
	var ok bool
	var body string
	for time.Now().Before(deadline) {
		time.Sleep(500 * time.Millisecond)
		ok, body = agent.Health(req.ID)
		if ok { break }
	}
	respond(w, 200, map[string]interface{}{"started": true, "healthy": ok, "body": body})
}

func handleAgentStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ ID string `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	agent.Stop(req.ID)
	respond(w, 200, map[string]string{"status": "stopped"})
}

func handleAgentUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
	var req struct{ ID string `json:"id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { jsonError(w, 400, "invalid JSON"); return }
	if err := agent.Uninstall(req.ID); err != nil { jsonError(w, 500, err.Error()); return }
	respond(w, 200, map[string]string{"status": "uninstalled"})
}

func handleAgentHealth(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" { jsonError(w, 400, "id required"); return }
	ok, body := agent.Health(id)
	respond(w, 200, map[string]interface{}{"ok": ok, "body": body})
}

// ── Ollama ────────────────────────────────────────────────────────────────────

func handleOllamaModels(w http.ResponseWriter, r *http.Request) {
	base := r.URL.Query().Get("url")
	if base == "" { base = "http://localhost:11434" }
	base = strings.TrimRight(base, "/")

	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(base + "/api/tags")
	if err != nil {
		respond(w, 200, map[string]interface{}{"models": []interface{}{}, "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		respond(w, 200, map[string]interface{}{"models": []interface{}{}, "error": "risposta Ollama non valida: " + err.Error()})
		return
	}

	out := make([]map[string]string, len(tags.Models))
	for i, m := range tags.Models {
		out[i] = map[string]string{"id": m.Name, "label": m.Name}
	}
	respond(w, 200, map[string]interface{}{"models": out})
}
