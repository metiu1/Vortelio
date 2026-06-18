package cloud

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// Media cloud generation (image / audio / video / 3D) via BYOK APIs. fal.ai is a
// near-universal provider (one API, model id selects the task), complemented by
// OpenAI (image + TTS) and ElevenLabs (TTS). Keys are stored like LLM keys.

// MediaProvider describes a configurable cloud media service.
type MediaProvider struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"` // image | audio | video | 3d
	DefaultModel string `json:"default_model"`
	KeyHint      string `json:"key_hint"`
}

// MediaProviders is the registry shown in "Add cloud model" for media types.
var MediaProviders = []MediaProvider{
	// Image
	{ID: "openai_image", Name: "OpenAI (DALL·E)", Type: "image", DefaultModel: "dall-e-3", KeyHint: "https://platform.openai.com/api-keys"},
	{ID: "stability", Name: "Stability AI", Type: "image", DefaultModel: "sd3.5-large", KeyHint: "https://platform.stability.ai/account/keys"},
	{ID: "fal_image", Name: "fal.ai (FLUX)", Type: "image", DefaultModel: "fal-ai/flux/schnell", KeyHint: "https://fal.ai/dashboard/keys"},
	// Audio (text-to-speech)
	{ID: "openai_audio", Name: "OpenAI TTS", Type: "audio", DefaultModel: "tts-1", KeyHint: "https://platform.openai.com/api-keys"},
	{ID: "elevenlabs", Name: "ElevenLabs", Type: "audio", DefaultModel: "eleven_multilingual_v2", KeyHint: "https://elevenlabs.io/app/settings/api-keys"},
	// Video
	{ID: "fal_video", Name: "fal.ai (Video)", Type: "video", DefaultModel: "fal-ai/ltx-video", KeyHint: "https://fal.ai/dashboard/keys"},
	// 3D
	{ID: "fal_3d", Name: "fal.ai (3D)", Type: "3d", DefaultModel: "fal-ai/triposr", KeyHint: "https://fal.ai/dashboard/keys"},
}

func FindMediaProvider(id string) (MediaProvider, bool) {
	for _, p := range MediaProviders {
		if p.ID == id {
			return p, true
		}
	}
	return MediaProvider{}, false
}

// ConfiguredMediaProvider returns the first media provider of the given type that
// has an API key set (so the agent can prefer cloud media when available).
func ConfiguredMediaProvider(mediaType string) (MediaProvider, string, bool) {
	for _, p := range MediaProviders {
		if p.Type != mediaType {
			continue
		}
		if key := LoadKey(p.ID); key != "" {
			return p, key, true
		}
	}
	return MediaProvider{}, "", false
}

var mediaHTTP = &http.Client{Timeout: 180 * time.Second}

// GenerateMedia runs a cloud media generation and returns the raw bytes plus a
// file extension. prompt is the text prompt; model overrides the provider default.
func GenerateMedia(providerID, key, model, prompt string) ([]byte, string, error) {
	p, ok := FindMediaProvider(providerID)
	if !ok {
		return nil, "", fmt.Errorf("provider media sconosciuto: %s", providerID)
	}
	if model == "" {
		model = p.DefaultModel
	}
	switch providerID {
	case "openai_image":
		return genOpenAIImage(key, model, prompt)
	case "stability":
		return genStabilityImage(key, model, prompt)
	case "openai_audio":
		return genOpenAITTS(key, model, prompt)
	case "elevenlabs":
		return genElevenLabs(key, model, prompt)
	default:
		// fal.ai providers (image/video/3d) share one API.
		if strings.HasPrefix(providerID, "fal") {
			return genFal(key, model, prompt, p.Type)
		}
	}
	return nil, "", fmt.Errorf("provider non supportato: %s", providerID)
}

func genOpenAIImage(key, model, prompt string) ([]byte, string, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"model": model, "prompt": prompt, "n": 1, "size": "1024x1024", "response_format": "b64_json",
	})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := mediaHTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("OpenAI: %s", apiErr(raw))
	}
	var out struct {
		Data []struct {
			B64 string `json:"b64_json"`
			URL string `json:"url"`
		} `json:"data"`
	}
	json.Unmarshal(raw, &out)
	if len(out.Data) == 0 {
		return nil, "", fmt.Errorf("nessuna immagine restituita")
	}
	if out.Data[0].B64 != "" {
		b, err := base64.StdEncoding.DecodeString(out.Data[0].B64)
		return b, "png", err
	}
	return download(out.Data[0].URL, "png")
}

func genStabilityImage(key, model, prompt string) ([]byte, string, error) {
	// Stability v2beta uses multipart/form-data.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("prompt", prompt)
	mw.WriteField("model", model)
	mw.WriteField("output_format", "png")
	mw.Close()
	req, _ := http.NewRequest("POST", "https://api.stability.ai/v2beta/stable-image/generate/core", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "image/*")
	resp, err := mediaHTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("stability.ai: %s", apiErr(raw))
	}
	return raw, "png", nil
}

func genOpenAITTS(key, model, prompt string) ([]byte, string, error) {
	body, _ := json.Marshal(map[string]interface{}{"model": model, "input": prompt, "voice": "alloy"})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/audio/speech", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := mediaHTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("OpenAI TTS: %s", apiErr(raw))
	}
	return raw, "mp3", nil
}

func genElevenLabs(key, model, prompt string) ([]byte, string, error) {
	// Default voice "Rachel".
	voiceID := "21m00Tcm4TlvDq8ikWAM"
	body, _ := json.Marshal(map[string]interface{}{"text": prompt, "model_id": model})
	req, _ := http.NewRequest("POST", "https://api.elevenlabs.io/v1/text-to-speech/"+voiceID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", key)
	resp, err := mediaHTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("ElevenLabs: %s", apiErr(raw))
	}
	return raw, "mp3", nil
}

// genFal calls fal.run synchronously and downloads the first output asset.
func genFal(key, model, prompt, mediaType string) ([]byte, string, error) {
	body, _ := json.Marshal(map[string]interface{}{"prompt": prompt})
	req, _ := http.NewRequest("POST", "https://fal.run/"+model, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Key "+key)
	resp, err := mediaHTTP.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("fal.ai: %s", apiErr(raw))
	}
	url, ext := falAssetURL(raw, mediaType)
	if url == "" {
		return nil, "", fmt.Errorf("fal.ai: nessun output (%s)", truncate(string(raw), 200))
	}
	return download(url, ext)
}

// falAssetURL pulls the first asset URL out of a fal response for the media type.
func falAssetURL(raw []byte, mediaType string) (string, string) {
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		return "", ""
	}
	pick := func(v interface{}) string {
		switch t := v.(type) {
		case map[string]interface{}:
			if u, ok := t["url"].(string); ok {
				return u
			}
		case []interface{}:
			if len(t) > 0 {
				if mm, ok := t[0].(map[string]interface{}); ok {
					if u, ok := mm["url"].(string); ok {
						return u
					}
				}
			}
		}
		return ""
	}
	switch mediaType {
	case "image":
		if u := pick(m["images"]); u != "" {
			return u, "png"
		}
		if u := pick(m["image"]); u != "" {
			return u, "png"
		}
	case "video":
		if u := pick(m["video"]); u != "" {
			return u, "mp4"
		}
	case "3d":
		if u := pick(m["model_mesh"]); u != "" {
			return u, "glb"
		}
		if u := pick(m["mesh"]); u != "" {
			return u, "glb"
		}
	case "audio":
		if u := pick(m["audio"]); u != "" {
			return u, "mp3"
		}
	}
	return "", ""
}

func download(url, ext string) ([]byte, string, error) {
	resp, err := mediaHTTP.Get(url)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, ext, err
}

func apiErr(raw []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	json.Unmarshal(raw, &e)
	if e.Error.Message != "" {
		return e.Error.Message
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Detail != "" {
		return e.Detail
	}
	return truncate(string(raw), 200)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
