package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
	rt "github.com/vortelio/vortelio/internal/runtime"
)

// ── Model-as-tool media provider ─────────────────────────────────────────────
//
// Lets an LLM (local or cloud) call Vortelio's other generative models as tools:
// image, video, audio (TTS/STT) and 3D. Each tool resolves a locally installed
// model of the requested type (the caller may name one, otherwise the first
// installed model of that type is used), runs it, saves the artifact to the
// default output directory, and returns the path. A "media_generated" tool event
// carries a base64 data URI so the UI can render the result inline without
// bloating the LLM context with binary data.

type mediaProvider struct {
	emit rt.ToolEventEmitter
}

func newMediaProvider(emit rt.ToolEventEmitter) *mediaProvider {
	return &mediaProvider{emit: emit}
}

func (m *mediaProvider) Tools() []rt.ToolDef {
	return []rt.ToolDef{
		toolDef("generate_image", "Generate an image from a text prompt using a local image model (e.g. Stable Diffusion). Returns the saved file path.",
			`{"type":"object","properties":{"prompt":{"type":"string","description":"What to draw."},"model":{"type":"string","description":"Optional image model name. Defaults to the first installed image model."},"steps":{"type":"integer","description":"Sampling steps. Default 20."}},"required":["prompt"]}`),
		toolDef("generate_video", "Generate a short video clip from a text prompt using a local video model. Returns the saved file path.",
			`{"type":"object","properties":{"prompt":{"type":"string","description":"What the video should show."},"model":{"type":"string","description":"Optional video model name."},"steps":{"type":"integer","description":"Sampling steps. Default 20."}},"required":["prompt"]}`),
		toolDef("text_to_speech", "Synthesize speech audio from text using a local TTS model (e.g. Kokoro/Bark). Returns the saved audio file path.",
			`{"type":"object","properties":{"text":{"type":"string","description":"Text to speak."},"model":{"type":"string","description":"Optional audio model name."}},"required":["text"]}`),
		toolDef("transcribe", "Transcribe an audio file to text using a local speech-to-text model (e.g. Whisper).",
			`{"type":"object","properties":{"input_file":{"type":"string","description":"Path to the audio file to transcribe."},"model":{"type":"string","description":"Optional audio model name."}},"required":["input_file"]}`),
		toolDef("generate_3d", "Generate a 3D model (mesh) from a text prompt or an input image using a local 3D model. Returns the saved file path.",
			`{"type":"object","properties":{"prompt":{"type":"string","description":"Text prompt describing the object."},"input_file":{"type":"string","description":"Optional input image path for image-to-3D."},"model":{"type":"string","description":"Optional 3D model name."}},"required":[]}`),
	}
}

func (m *mediaProvider) Execute(name, argsJSON string) (string, error) {
	switch name {
	case "generate_image":
		return m.generateImage(argsJSON)
	case "generate_video":
		return m.generateVideo(argsJSON)
	case "text_to_speech":
		return m.textToSpeech(argsJSON)
	case "transcribe":
		return m.transcribe(argsJSON)
	case "generate_3d":
		return m.generate3D(argsJSON)
	default:
		return "", fmt.Errorf("unknown media tool: %s", name)
	}
}

// findMediaModel returns an installed model of the given type. If name is set it
// matches by Name/DisplayName (substring, case-insensitive); otherwise the first
// installed model of that type is returned.
func findMediaModel(typ, name string) (*hub.Model, error) {
	models, err := hub.NewModelStore().List()
	if err != nil {
		return nil, err
	}
	name = strings.ToLower(strings.TrimSpace(name))
	var first *hub.Model
	for _, mdl := range models {
		if mdl.Type != typ {
			continue
		}
		if first == nil {
			first = mdl
		}
		if name == "" {
			return mdl, nil
		}
		if strings.Contains(strings.ToLower(mdl.Name), name) ||
			strings.Contains(strings.ToLower(mdl.DisplayName), name) {
			return mdl, nil
		}
	}
	if name != "" && first != nil {
		return first, nil // requested name not found; fall back to first available
	}
	if first != nil {
		return first, nil
	}
	return nil, fmt.Errorf("no installed %s model found — download one first (vortelio pull %s:...)", typ, typ)
}

// saveArtifact writes data to the default output dir with a timestamped name and
// emits a media_generated event carrying a data URI for inline UI rendering.
func (m *mediaProvider) saveArtifact(kind, ext, mime string, data []byte) (string, error) {
	dir := rt.DefaultOutputDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	fname := fmt.Sprintf("vortelio-%s-%d.%s", kind, time.Now().Unix(), ext)
	path := filepath.Join(dir, fname)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	if m.emit != nil {
		dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
		m.emit("media_generated", map[string]interface{}{
			"kind": kind, "path": path, "mime": mime, "data_uri": dataURI,
		})
	}
	b, _ := json.Marshal(map[string]interface{}{
		"status": "ok", "kind": kind, "path": path,
		"note": "Artifact saved. The user can view it inline; reference it by path if needed.",
	})
	return string(b), nil
}

func (m *mediaProvider) generateImage(argsJSON string) (string, error) {
	var a struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
		Steps  int    `json:"steps"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	if a.Steps <= 0 {
		a.Steps = 20
	}
	mdl, err := findMediaModel("image", a.Model)
	if err != nil {
		return "", err
	}
	runner := rt.NewImageRunner(mdl, getHardware())
	data, err := runner.GenerateToBytes(a.Prompt, a.Steps, false)
	if err != nil {
		return "", err
	}
	return m.saveArtifact("image", "png", "image/png", data)
}

func (m *mediaProvider) textToSpeech(argsJSON string) (string, error) {
	var a struct {
		Text  string `json:"text"`
		Model string `json:"model"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Text) == "" {
		return "", fmt.Errorf("text is required")
	}
	mdl, err := findMediaModel("audio", a.Model)
	if err != nil {
		return "", err
	}
	runner := rt.NewAudioRunner(mdl, getHardware())
	data, err := runner.SynthesizeToBytes(a.Text)
	if err != nil {
		return "", err
	}
	return m.saveArtifact("audio", "wav", "audio/wav", data)
}

func (m *mediaProvider) transcribe(argsJSON string) (string, error) {
	var a struct {
		InputFile string `json:"input_file"`
		Model     string `json:"model"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.InputFile) == "" {
		return "", fmt.Errorf("input_file is required")
	}
	if _, err := os.Stat(a.InputFile); err != nil {
		return "", fmt.Errorf("input file not found: %s", a.InputFile)
	}
	mdl, err := findMediaModel("audio", a.Model)
	if err != nil {
		return "", err
	}
	runner := rt.NewAudioRunner(mdl, getHardware())
	text, err := runner.TranscribeText(a.InputFile)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(map[string]interface{}{"status": "ok", "text": text})
	return string(b), nil
}

func (m *mediaProvider) generateVideo(argsJSON string) (string, error) {
	var a struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
		Steps  int    `json:"steps"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Prompt) == "" {
		return "", fmt.Errorf("prompt is required")
	}
	if a.Steps <= 0 {
		a.Steps = 20
	}
	mdl, err := findMediaModel("video", a.Model)
	if err != nil {
		return "", err
	}
	runner := rt.NewVideoRunner(mdl, getHardware())
	data, err := m.runToFile(runner, &rt.RunOptions{Prompt: a.Prompt, Steps: a.Steps}, "mp4")
	if err != nil {
		return "", err
	}
	return m.saveArtifact("video", "mp4", "video/mp4", data)
}

func (m *mediaProvider) generate3D(argsJSON string) (string, error) {
	var a struct {
		Prompt    string `json:"prompt"`
		InputFile string `json:"input_file"`
		Model     string `json:"model"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Prompt) == "" && strings.TrimSpace(a.InputFile) == "" {
		return "", fmt.Errorf("provide a prompt or an input_file")
	}
	mdl, err := findMediaModel("3d", a.Model)
	if err != nil {
		return "", err
	}
	runner := rt.NewThreeDRunner(mdl, getHardware())
	data, err := m.runToFile(runner, &rt.RunOptions{Prompt: a.Prompt, InputFile: a.InputFile}, "obj")
	if err != nil {
		return "", err
	}
	return m.saveArtifact("3d", "obj", "model/obj", data)
}

// runToFile runs a runner with a temp output file and returns the bytes produced.
func (m *mediaProvider) runToFile(runner rt.Runner, opts *rt.RunOptions, ext string) ([]byte, error) {
	tmp, err := os.CreateTemp("", "vortelio-tool-*."+ext)
	if err != nil {
		return nil, err
	}
	tmp.Close()
	opts.OutputFile = tmp.Name()
	defer os.Remove(tmp.Name())
	if err := runner.Run(opts); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}
