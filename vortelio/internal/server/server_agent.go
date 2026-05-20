package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/agent"
	"github.com/vortelio/vortelio/internal/config"
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

// ── CrewAI orchestration ──────────────────────────────────────────────────────

func crewsDir() string {
	return filepath.Join(config.HomeDir(), "crews")
}

func handleCrewList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet { jsonError(w, 405, "GET only"); return }
	os.MkdirAll(crewsDir(), 0755)
	entries, _ := os.ReadDir(crewsDir())
	crews := []map[string]interface{}{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(crewsDir(), e.Name()))
		if err != nil {
			continue
		}
		var crew map[string]interface{}
		if json.Unmarshal(data, &crew) == nil {
			crew["name"] = strings.TrimSuffix(e.Name(), ".json")
			crews = append(crews, crew)
		}
	}
	respond(w, 200, map[string]interface{}{"crews": crews})
}

// handleCrewDispatch routes /api/crewai/crews/{name} and /api/crewai/crews/{name}/run
func handleCrewDispatch(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/crewai/crews/")
	path = strings.TrimSuffix(path, "/")

	if strings.HasSuffix(path, "/run") {
		name := strings.TrimSuffix(path, "/run")
		if r.Method != http.MethodPost { jsonError(w, 405, "POST only"); return }
		handleCrewRun(w, r, name)
		return
	}

	name := path
	if name == "" { jsonError(w, 400, "name required"); return }

	switch r.Method {
	case http.MethodGet:
		handleCrewGet(w, r, name)
	case http.MethodPost, http.MethodPut:
		handleCrewSave(w, r, name)
	case http.MethodDelete:
		handleCrewDelete(w, r, name)
	default:
		jsonError(w, 405, "method not allowed")
	}
}

func handleCrewGet(w http.ResponseWriter, r *http.Request, name string) {
	if !validCrewName(name) { jsonError(w, 400, "nome non valido"); return }
	data, err := os.ReadFile(filepath.Join(crewsDir(), name+".json"))
	if err != nil { jsonError(w, 404, "crew non trovata"); return }
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func handleCrewSave(w http.ResponseWriter, r *http.Request, name string) {
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil { jsonError(w, 400, "invalid JSON"); return }
	// Allow name from URL to override body
	if name != "" {
		body["name"] = name
	} else {
		n, _ := body["name"].(string)
		name = strings.TrimSpace(n)
	}
	if !validCrewName(name) { jsonError(w, 400, "nome crew non valido"); return }
	os.MkdirAll(crewsDir(), 0755)
	data, _ := json.MarshalIndent(body, "", "  ")
	os.WriteFile(filepath.Join(crewsDir(), name+".json"), data, 0644)
	respond(w, 200, map[string]interface{}{"ok": true, "name": name})
}

func handleCrewDelete(w http.ResponseWriter, r *http.Request, name string) {
	if !validCrewName(name) { jsonError(w, 400, "nome non valido"); return }
	os.Remove(filepath.Join(crewsDir(), name+".json"))
	respond(w, 200, map[string]interface{}{"ok": true})
}

func handleCrewRun(w http.ResponseWriter, r *http.Request, name string) {
	if !validCrewName(name) { jsonError(w, 400, "nome non valido"); return }

	// Proxy run to CrewAI Python server (port 8500)
	var reqBody map[string]interface{}
	json.NewDecoder(r.Body).Decode(&reqBody)
	bodyBytes, _ := json.Marshal(reqBody)

	target := "http://localhost:8500/api/crews/" + name + "/run"
	hreq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(bodyBytes))
	if err != nil { jsonError(w, 500, "cannot build request: "+err.Error()); return }
	hreq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Minute, Transport: &http.Transport{DisableCompression: true}}
	resp, err := client.Do(hreq)
	if err != nil {
		jsonError(w, 503, "CrewAI server non disponibile — avvia prima l'agente CrewAI: "+err.Error())
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
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

func validCrewName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "/\\") && !strings.Contains(name, "..")
}

// handleCrewStudioProxy forwards /api/crewai/studio/* to the Python crewai-studio server at port 8500.
func handleCrewStudioProxy(w http.ResponseWriter, r *http.Request) {
	subpath := strings.TrimPrefix(r.URL.Path, "/api/crewai/studio")
	if subpath == "" {
		subpath = "/"
	}
	target := "http://localhost:8500" + subpath
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	hreq, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		jsonError(w, 500, "proxy build error: "+err.Error())
		return
	}
	for k, v := range r.Header {
		if strings.EqualFold(k, "host") {
			continue
		}
		hreq.Header[k] = v
	}

	client := &http.Client{
		Timeout:   30 * time.Minute,
		Transport: &http.Transport{DisableCompression: true},
	}
	resp, err := client.Do(hreq)
	if err != nil {
		jsonError(w, 503, "CrewAI Studio non disponibile — avvia prima l'agente CrewAI: "+err.Error())
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(resp.StatusCode)

	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 1024)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if canFlush {
				flusher.Flush()
			}
		}
		if rerr != nil {
			break
		}
	}
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
