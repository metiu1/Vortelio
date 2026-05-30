package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

// ─────────────────────────────────────────────────────────────────────────────
// /api/hooks — webhook management
// ─────────────────────────────────────────────────────────────────────────────

type Webhook struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"` // "pull_done", "model_loaded", "generate_done", "*"
	Secret string   `json:"secret,omitempty"`
}

var (
	hooksMu     sync.RWMutex
	hooks       []Webhook
	hooksLoaded bool
)

func hooksFile() string {
	return filepath.Join(config.HomeDir(), "webhooks.json")
}

func loadHooks() {
	hooksMu.Lock()
	defer hooksMu.Unlock()
	if hooksLoaded {
		return
	}
	hooksLoaded = true
	data, err := os.ReadFile(hooksFile())
	if err == nil {
		json.Unmarshal(data, &hooks)
	}
}

func saveHooks() {
	os.MkdirAll(config.HomeDir(), 0755)
	data, _ := json.MarshalIndent(hooks, "", "  ")
	os.WriteFile(hooksFile(), data, 0644)
}

// FireHook sends event payload to all matching webhooks asynchronously.
func FireHook(event string, payload map[string]interface{}) {
	loadHooks()
	hooksMu.RLock()
	matched := make([]Webhook, 0, len(hooks))
	for _, h := range hooks {
		for _, e := range h.Events {
			if e == "*" || e == event {
				matched = append(matched, h)
				break
			}
		}
	}
	hooksMu.RUnlock()

	if len(matched) == 0 {
		return
	}
	payload["event"] = event
	payload["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	body, _ := json.Marshal(payload)

	for _, h := range matched {
		go func(h Webhook) {
			req, _ := http.NewRequest("POST", h.URL, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Vortelio-Event", event)
			if h.Secret != "" {
				req.Header.Set("X-Vortelio-Secret", h.Secret)
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err == nil {
				resp.Body.Close()
			}
		}(h)
	}
}

func handleHooks(w http.ResponseWriter, r *http.Request) {
	loadHooks()
	switch r.Method {
	case http.MethodGet:
		hooksMu.RLock()
		defer hooksMu.RUnlock()
		respond(w, 200, map[string]interface{}{"hooks": hooks})

	case http.MethodPost:
		var h Webhook
		if err := json.NewDecoder(r.Body).Decode(&h); err != nil {
			jsonError(w, 400, "invalid JSON: "+err.Error())
			return
		}
		if h.URL == "" {
			jsonError(w, 400, "url required")
			return
		}
		if h.ID == "" {
			h.ID = fmt.Sprintf("hk_%d", time.Now().UnixNano())
		}
		if len(h.Events) == 0 {
			h.Events = []string{"*"}
		}
		hooksMu.Lock()
		// upsert by ID
		updated := false
		for i, existing := range hooks {
			if existing.ID == h.ID {
				hooks[i] = h
				updated = true
				break
			}
		}
		if !updated {
			hooks = append(hooks, h)
		}
		saveHooks()
		hooksMu.Unlock()
		respond(w, 200, h)

	case http.MethodDelete:
		id := strings.TrimPrefix(r.URL.Path, "/api/hooks/")
		if id == "" || id == "/api/hooks" {
			var body struct {
				ID string `json:"id"`
			}
			json.NewDecoder(r.Body).Decode(&body)
			id = body.ID
		}
		if id == "" {
			jsonError(w, 400, "id required")
			return
		}
		hooksMu.Lock()
		out := hooks[:0]
		for _, h := range hooks {
			if h.ID != id {
				out = append(out, h)
			}
		}
		hooks = out
		saveHooks()
		hooksMu.Unlock()
		respond(w, 200, map[string]string{"status": "deleted", "id": id})

	default:
		jsonError(w, 405, "GET/POST/DELETE only")
	}
}
