package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// handleHFSearch searches HuggingFace for models (GGUF-focused) so users can find
// and download them straight from Vortelio. GET /api/hf/search?q=...
func handleHFSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		respond(w, 200, map[string]interface{}{"models": []interface{}{}})
		return
	}
	api := "https://huggingface.co/api/models?limit=25&full=false&sort=downloads&direction=-1&search=" + url.QueryEscape(q)
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", api, nil)
	req.Header.Set("User-Agent", "Vortelio")
	resp, err := client.Do(req)
	if err != nil {
		jsonError(w, 502, "HuggingFace non raggiungibile: "+err.Error())
		return
	}
	defer resp.Body.Close()

	var raw []struct {
		ID           string   `json:"id"`
		Likes        int      `json:"likes"`
		Downloads    int      `json:"downloads"`
		PipelineTag  string   `json:"pipeline_tag"`
		Tags         []string `json:"tags"`
		LibraryName  string   `json:"library_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		jsonError(w, 502, "risposta HuggingFace non valida")
		return
	}

	type modelOut struct {
		ID        string `json:"id"`
		Downloads int    `json:"downloads"`
		Likes     int    `json:"likes"`
		Task      string `json:"task"`
		GGUF      bool   `json:"gguf"`
		Type      string `json:"type"`
	}
	out := make([]modelOut, 0, len(raw))
	for _, m := range raw {
		gguf := strings.EqualFold(m.LibraryName, "gguf")
		for _, t := range m.Tags {
			if strings.EqualFold(t, "gguf") {
				gguf = true
			}
		}
		out = append(out, modelOut{
			ID:        m.ID,
			Downloads: m.Downloads,
			Likes:     m.Likes,
			Task:      m.PipelineTag,
			GGUF:      gguf,
			Type:      hfTypeFor(m.PipelineTag, m.Tags),
		})
	}
	respond(w, 200, map[string]interface{}{"models": out})
}

// hfTypeFor maps a HF pipeline tag to a Vortelio model type prefix.
func hfTypeFor(task string, tags []string) string {
	t := strings.ToLower(task)
	switch {
	case strings.Contains(t, "text-to-image") || strings.Contains(t, "image"):
		return "image"
	case strings.Contains(t, "text-to-video") || strings.Contains(t, "video"):
		return "video"
	case strings.Contains(t, "text-to-speech") || strings.Contains(t, "audio") || strings.Contains(t, "automatic-speech"):
		return "audio"
	case strings.Contains(t, "to-3d") || strings.Contains(t, "3d"):
		return "3d"
	default:
		return "llm"
	}
}
