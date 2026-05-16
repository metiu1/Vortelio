package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
)

// ── POST /v1/audio/transcriptions ────────────────────────────────────────────

func handleOpenAIAudioTranscriptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		jsonError(w, 400, "multipart parse error: "+err.Error())
		return
	}

	modelName := r.FormValue("model")
	if modelName == "" {
		modelName = "audio/whisper:large"
	}
	responseFormat := r.FormValue("response_format")
	if responseFormat == "" {
		responseFormat = "json"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, 400, "missing file field: "+err.Error())
		return
	}
	defer file.Close()

	ext := ".wav"
	if idx := strings.LastIndex(header.Filename, "."); idx >= 0 {
		ext = header.Filename[idx:]
	}
	tmp, err := os.CreateTemp("", "vortelio-stt-*"+ext)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		jsonError(w, 500, "failed to save upload: "+err.Error())
		return
	}
	tmp.Close()

	model, err := resolveAudioModel(modelName)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	hw := getHardware()
	ar := runtime.NewAudioRunner(model, hw)

	text, err := ar.TranscribeText(tmpPath)
	if err != nil {
		jsonError(w, 500, "transcription failed: "+err.Error())
		return
	}

	switch responseFormat {
	case "text":
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, text)
	case "srt":
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "1\n00:00:00,000 --> 00:00:10,000\n%s\n\n", text)
	case "vtt":
		w.Header().Set("Content-Type", "text/vtt")
		fmt.Fprintf(w, "WEBVTT\n\n00:00:00.000 --> 00:00:10.000\n%s\n\n", text)
	default: // "json", "verbose_json"
		respond(w, 200, map[string]interface{}{
			"text": text,
		})
	}
}

// ── POST /v1/audio/translations ───────────────────────────────────────────────

func handleOpenAIAudioTranslations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	if err := r.ParseMultipartForm(200 << 20); err != nil {
		jsonError(w, 400, "multipart parse error: "+err.Error())
		return
	}

	modelName := r.FormValue("model")
	if modelName == "" {
		modelName = "audio/whisper:large"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, 400, "missing file field: "+err.Error())
		return
	}
	defer file.Close()

	ext := ".wav"
	if idx := strings.LastIndex(header.Filename, "."); idx >= 0 {
		ext = header.Filename[idx:]
	}
	tmp, err := os.CreateTemp("", "vortelio-trans-*"+ext)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, file); err != nil {
		tmp.Close()
		jsonError(w, 500, "failed to save upload: "+err.Error())
		return
	}
	tmp.Close()

	model, err := resolveAudioModel(modelName)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	hw := getHardware()
	ar := runtime.NewAudioRunner(model, hw)

	text, err := ar.TranslateText(tmpPath)
	if err != nil {
		jsonError(w, 500, "translation failed: "+err.Error())
		return
	}

	respond(w, 200, map[string]interface{}{"text": text})
}

// ── POST /v1/audio/speech ─────────────────────────────────────────────────────

type openAISpeechRequest struct {
	Model          string  `json:"model"`
	Input          string  `json:"input"`
	Voice          string  `json:"voice"`
	ResponseFormat string  `json:"response_format"`
	Speed          float64 `json:"speed"`
}

func handleOpenAIAudioSpeech(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req openAISpeechRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if req.Input == "" {
		jsonError(w, 400, "input is required")
		return
	}
	modelName := req.Model
	if modelName == "" {
		modelName = "audio/kokoro"
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "wav"
	}

	model, err := resolveAudioModel(modelName)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	hw := getHardware()
	ar := runtime.NewAudioRunner(model, hw)

	data, err := ar.SynthesizeToBytes(req.Input)
	if err != nil {
		jsonError(w, 500, "synthesis failed: "+err.Error())
		return
	}

	mimeTypes := map[string]string{
		"wav":  "audio/wav",
		"mp3":  "audio/mpeg",
		"ogg":  "audio/ogg",
		"flac": "audio/flac",
	}
	mime := mimeTypes[req.ResponseFormat]
	if mime == "" {
		mime = "audio/wav"
	}
	w.Header().Set("Content-Type", mime)
	w.WriteHeader(200)
	w.Write(data)
}

// ── POST /v1/images/generations ───────────────────────────────────────────────

type openAIImageGenerationRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size"`
	ResponseFormat string `json:"response_format"`
	Quality        string `json:"quality"`
	Style          string `json:"style"`
}

func handleOpenAIImageGenerations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req openAIImageGenerationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if req.Prompt == "" {
		jsonError(w, 400, "prompt is required")
		return
	}
	if req.N <= 0 {
		req.N = 1
	}
	if req.ResponseFormat == "" {
		req.ResponseFormat = "url"
	}
	modelName := req.Model
	if modelName == "" {
		modelName = "image/sdxl"
	}

	model, err := resolveImageModel(modelName)
	if err != nil {
		jsonError(w, 404, err.Error())
		return
	}
	hw := getHardware()
	ir := runtime.NewImageRunner(model, hw)

	type imgResult struct {
		B64JSON string `json:"b64_json,omitempty"`
		URL     string `json:"url,omitempty"`
	}
	var results []imgResult

	for i := 0; i < req.N; i++ {
		data, err := ir.GenerateToBytes(req.Prompt, 20, false)
		if err != nil {
			jsonError(w, 500, fmt.Sprintf("generation failed: %v", err))
			return
		}
		if req.ResponseFormat == "b64_json" {
			results = append(results, imgResult{B64JSON: base64.StdEncoding.EncodeToString(data)})
		} else {
			// Save to output dir and return path as "url"
			outDir := runtime.DefaultOutputDir()
			outPath := fmt.Sprintf("%s/vortelio-img-%d.png", outDir, time.Now().UnixNano())
			os.WriteFile(outPath, data, 0644)
			results = append(results, imgResult{URL: "file://" + outPath})
		}
	}

	respond(w, 200, map[string]interface{}{
		"created": time.Now().Unix(),
		"data":    results,
	})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func resolveAudioModel(name string) (*hub.Model, error) {
	if !strings.Contains(name, "/") {
		name = "audio/" + name
	}
	ref, err := hub.ParseModelRef(name)
	if err != nil {
		return nil, err
	}
	store := hub.NewModelStore()
	model, err := store.Resolve(ref)
	if err != nil {
		// Try fuzzy: any audio model
		models, lerr := store.List()
		if lerr == nil {
			for _, m := range models {
				if m.Type == "audio" {
					return m, nil
				}
			}
		}
		return nil, fmt.Errorf("audio model %q not found", name)
	}
	return model, nil
}

func resolveImageModel(name string) (*hub.Model, error) {
	if !strings.Contains(name, "/") {
		name = "image/" + name
	}
	ref, err := hub.ParseModelRef(name)
	if err != nil {
		return nil, err
	}
	store := hub.NewModelStore()
	model, err := store.Resolve(ref)
	if err != nil {
		models, lerr := store.List()
		if lerr == nil {
			for _, m := range models {
				if m.Type == "image" {
					return m, nil
				}
			}
		}
		return nil, fmt.Errorf("image model %q not found", name)
	}
	return model, nil
}
