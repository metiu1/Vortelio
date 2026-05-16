package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vortelio/vortelio/internal/hub"
)

type AudioRunner struct {
	model *hub.Model
	hw    *Hardware
}

func NewAudioRunner(model *hub.Model, hw *Hardware) *AudioRunner {
	return &AudioRunner{model: model, hw: hw}
}

// ensureDeps installs required Python packages for the given model if missing.
func (r *AudioRunner) ensureDeps(pythonBin string) {
	name := strings.ToLower(r.model.Name)
	switch {
	case strings.Contains(name, "whisper"):
		if !CheckPythonPackage(pythonBin, "faster_whisper") {
			fmt.Println("📦  Installazione faster-whisper...")
			_ = InstallPythonPackage(pythonBin, "faster-whisper")
			if !CheckPythonPackage(pythonBin, "faster_whisper") {
				// fallback: openai-whisper
				fmt.Println("📦  Installazione openai-whisper (fallback)...")
				_ = InstallPythonPackage(pythonBin, "openai-whisper")
			}
		}
	case strings.Contains(name, "kokoro"):
		if !CheckPythonPackage(pythonBin, "kokoro_onnx") && !CheckPythonPackage(pythonBin, "kokoro") {
			fmt.Println("📦  Installazione kokoro-onnx soundfile...")
			_ = InstallPythonPackage(pythonBin, "kokoro-onnx", "soundfile")
		}
		if !CheckPythonPackage(pythonBin, "soundfile") {
			_ = InstallPythonPackage(pythonBin, "soundfile")
		}
	case strings.Contains(name, "bark"):
		if !CheckPythonPackage(pythonBin, "bark") {
			fmt.Println("📦  Installazione bark (potrebbe richiedere qualche minuto)...")
			_ = InstallPythonPackage(pythonBin, "git+https://github.com/suno-ai/bark.git", "scipy")
		}
		if !CheckPythonPackage(pythonBin, "torch") {
			fmt.Println("📦  Installazione torch...")
			_ = InstallPythonPackage(pythonBin, "torch")
		}
	}
}

func (r *AudioRunner) Run(opts *RunOptions) error {
	pythonBin := FindPython()
	if pythonBin == "" {
		fmt.Println("\n⚠️   Python 3 non trovato.")
		fmt.Println("    Installa Python 3.10+ da: https://python.org/downloads")
		return nil
	}
	r.ensureDeps(pythonBin)
	name := strings.ToLower(r.model.Name)
	switch {
	case strings.Contains(name, "whisper"):
		if opts.InputFile != "" { return r.runPython(pythonBin, r.buildWhisperScript(opts)) }
		fmt.Println("ℹ️   Whisper: Speech → Text")
		fmt.Println("    vortelio run audio/whisper:large --input ./audio.mp3")
		return nil
	case strings.Contains(name, "kokoro"):
		return r.runPython(pythonBin, r.buildKokoroScript(opts))
	case strings.Contains(name, "bark"):
		return r.runPython(pythonBin, r.buildBarkScript(opts))
	default:
		if opts.InputFile != "" { return r.runPython(pythonBin, r.buildWhisperScript(opts)) }
		return r.runPython(pythonBin, r.buildKokoroScript(opts))
	}
}

func (r *AudioRunner) RunCapture(opts *RunOptions) error {
	pythonBin := FindPython()
	if pythonBin == "" {
		return fmt.Errorf("Python 3 non trovato — installa Python 3.10+")
	}
	r.ensureDeps(pythonBin)
	name := strings.ToLower(r.model.Name)
	var script string
	switch {
	case strings.Contains(name, "whisper"):
		script = r.buildWhisperScript(opts)
	case strings.Contains(name, "kokoro"):
		script = r.buildKokoroScript(opts)
	default:
		script = r.buildBarkScript(opts)
	}
	return r.runPythonWith(pythonBin, script, true)
}

func (r *AudioRunner) RunWithProgress(opts *RunOptions, progress chan<- ProgressEvent) error {
	pythonBin := FindPython()
	if pythonBin == "" {
		if progress != nil { close(progress) }
		return fmt.Errorf("Python 3 non trovato")
	}
	r.ensureDeps(pythonBin)
	name := strings.ToLower(r.model.Name)
	var script string
	switch {
	case strings.Contains(name, "whisper"):
		script = r.buildWhisperScript(opts)
	case strings.Contains(name, "bark"):
		script = r.buildBarkScript(opts)
	default:
		script = r.buildKokoroScript(opts)
	}
	tmp, err := os.CreateTemp("", "vortelio-audio-*.py")
	if err != nil {
		if progress != nil { close(progress) }
		return err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := HideWindow(exec.Command(pythonBin, tmp.Name()))
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	return RunWithProgress(cmd, progress)
}

func (r *AudioRunner) runPython(pythonBin, script string) error {
	return r.runPythonWith(pythonBin, script, false)
}

func (r *AudioRunner) runPythonWith(pythonBin, script string, capture bool) error {
	tmp, err := os.CreateTemp("", "vortelio-audio-*.py")
	if err != nil { return err }
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := HideWindow(exec.Command(pythonBin, tmp.Name()))
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	if capture { return RunWithCapture(cmd) }
	return RunWithOutput(cmd, os.Stdout, os.Stderr)
}

func (r *AudioRunner) deviceString(forceCPU bool) string {
	if forceCPU { return "cpu" }
	switch r.hw.Backend {
	case BackendCUDA: return "cuda"
	case BackendMetal: return "mps"
	default: return "cpu"
	}
}

func (r *AudioRunner) buildKokoroScript(opts *RunOptions) string {
	text := opts.Prompt
	if text == "" { text = "Ciao, sono Vortelio." }
	outputPath := opts.OutputFile
	if outputPath == "" { outputPath = ResolveOutputPath("", "output.wav") }
	outputPath = strings.ReplaceAll(outputPath, `\`, `/`)
	device := r.deviceString(opts.ForceCPU)

	lines := []string{
		`import sys, os, pathlib, urllib.request`,
		`os.environ["PYTHONIOENCODING"] = "utf-8"`,
		``,
		`output_path = """` + outputPath + `"""`,
		`device = "` + device + `"`,
		`text = """` + escapePy(text) + `"""`,
		``,
		`# Try kokoro-onnx first (Python 3.14 compatible, no espeak-ng needed)`,
		`_lib = None`,
		`try:`,
		`    import kokoro_onnx`,
		`    _lib = "kokoro-onnx"`,
		`except ImportError:`,
		`    pass`,
		``,
		`# Fallback: kokoro (requires Python <= 3.12 and espeak-ng)`,
		`if _lib is None:`,
		`    try:`,
		`        from kokoro import KPipeline`,
		`        _lib = "kokoro"`,
		`    except ImportError:`,
		`        pass`,
		``,
		`if _lib is None:`,
		`    print("ERRORE: installa kokoro con:")`,
		`    print("  pip install kokoro-onnx soundfile  (consigliato, Python 3.14)")`,
		`    print("  pip install kokoro soundfile       (richiede Python <= 3.12)")`,
		`    sys.exit(1)`,
		``,
		`import soundfile as sf`,
		`import numpy as np`,
		``,
		`def _download_if_missing(url, dest):`,
		`    dest = pathlib.Path(dest)`,
		`    if not dest.exists():`,
		`        dest.parent.mkdir(parents=True, exist_ok=True)`,
		`        print("Download: " + dest.name + " ...")`,
		`        urllib.request.urlretrieve(url, dest)`,
		`    return str(dest)`,
		``,
		`_KOKORO_BASE = "https://github.com/thewh1teagle/kokoro-onnx/releases/download/model-files-v1.0"`,
		`_cache = pathlib.Path.home() / ".vortelio" / "models" / "audio" / "kokoro"`,
		``,
		`if _lib == "kokoro-onnx":`,
		`    print("Caricamento Kokoro ONNX...")`,
		`    model_path  = _download_if_missing(_KOKORO_BASE + "/kokoro-v1.0.onnx",  _cache / "kokoro-v1.0.onnx")`,
		`    voices_path = _download_if_missing(_KOKORO_BASE + "/voices-v1.0.bin",   _cache / "voices-v1.0.bin")`,
		`    tts = kokoro_onnx.Kokoro(model_path, voices_path)`,
		`    samples, sr = tts.create(text, voice="af_heart", speed=1.0, lang="en-us")`,
		`    sf.write(output_path, samples, sr)`,
		`else:`,
		`    print("Caricamento Kokoro su " + device + "...")`,
		`    pipe = KPipeline(lang_code="a", device=device)`,
		`    samples = []`,
		`    for i, (gs, ps, audio) in enumerate(pipe(text, voice="af_heart", speed=1.0)):`,
		`        samples.append(audio)`,
		`    audio = np.concatenate(samples)`,
		`    sf.write(output_path, audio, 24000)`,
		``,
		`print("Audio salvato in: " + output_path)`,
	}
	return strings.Join(lines, "\n") + "\n"
}


func (r *AudioRunner) buildWhisperScript(opts *RunOptions) string {
	inputPath := strings.ReplaceAll(opts.InputFile, `\`, `/`)
	device    := r.deviceString(opts.ForceCPU)
	tag       := r.model.Tag

	lines := []string{
		`import sys, os`,
		`os.environ["PYTHONIOENCODING"] = "utf-8"`,
		``,
		`_lib = None`,
		`try:`,
		`    from faster_whisper import WhisperModel as _FW`,
		`    _lib = "faster-whisper"`,
		`except ImportError:`,
		`    pass`,
		``,
		`if _lib is None:`,
		`    try:`,
		`        import whisper as _W`,
		`        _lib = "openai-whisper"`,
		`    except Exception:`,
		`        pass`,
		``,
		`if _lib is None:`,
		`    print("ERRORE: pip install faster-whisper")`,
		`    sys.exit(1)`,
		``,
		`try:`,
		`    import torch as _t`,
		`    _cuda_ok = _t.cuda.is_available()`,
		`    del _t`,
		`except ImportError:`,
		`    _cuda_ok = False`,
		``,
		`_dev = "cuda" if _cuda_ok and "` + device + `" != "cpu" else "cpu"`,
		``,
		`_tag_map = {`,
		`    "large": "large-v3", "large-v3": "large-v3", "large-v2": "large-v2",`,
		`    "large-v1": "large-v1", "medium": "medium", "small": "small",`,
		`    "base": "base", "tiny": "tiny", "turbo": "large-v3-turbo",`,
		`    "large-v3-turbo": "large-v3-turbo", "distil-large-v3": "distil-large-v3",`,
		`}`,
		`import re as _re`,
		`_base = _re.split(r"[/\\]", "` + tag + `")[-1].lower().strip()`,
		`_fw_model = _tag_map.get(_base, _tag_map.get("` + tag + `".lower(), "large-v3"))`,
		``,
		`if _lib == "faster-whisper":`,
		`    _ct = "float16" if _dev == "cuda" else "int8"`,
		`    print("Caricamento: " + _fw_model + " (" + _ct + ")")`,
		`    _m = _FW(_fw_model, device=_dev, compute_type=_ct)`,
		`    _segs, _ = _m.transcribe("""` + inputPath + `""", beam_size=5)`,
		`    _text = "".join(seg.text for seg in _segs)`,
		`else:`,
		`    _m = _W.load_model(_fw_model, device=_dev)`,
		`    _res = _W.transcribe(_m, """` + inputPath + `""", fp16=(_dev != "cpu"))`,
		`    _text = _res["text"]`,
		``,
		`print(_text)`,
	}
	script := strings.Join(lines, "\n") + "\n"
	if opts.OutputFile != "" {
		outPath := strings.ReplaceAll(ResolveOutputPath(opts.OutputFile, ""), `\`, `/`)
		script += "\nwith open(\"\"\"" + outPath + "\"\"\", \"w\", encoding=\"utf-8\") as f:\n"
		script += "    f.write(_text)\n"
		script += "print(\"Salvato in: " + outPath + "\")\n"
	}
	return script
}

// TranscribeText runs Whisper on inputFile and returns the transcribed text.
func (r *AudioRunner) TranscribeText(inputFile string) (string, error) {
	pythonBin := FindPython()
	if pythonBin == "" {
		return "", fmt.Errorf("Python 3 not found")
	}
	r.ensureDeps(pythonBin)
	opts := &RunOptions{InputFile: inputFile}
	script := r.buildWhisperScript(opts)
	tmp, err := os.CreateTemp("", "vortelio-whisper-*.py")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := HideWindow(exec.Command(pythonBin, tmp.Name()))
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("transcription failed: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	// The last non-empty line is the transcribed text
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" && !strings.HasPrefix(l, "Caricamento") && !strings.HasPrefix(l, "Salvato") {
			return l, nil
		}
	}
	return strings.TrimSpace(string(out)), nil
}

// TranslateText runs Whisper translate task on inputFile and returns English text.
func (r *AudioRunner) TranslateText(inputFile string) (string, error) {
	pythonBin := FindPython()
	if pythonBin == "" {
		return "", fmt.Errorf("Python 3 not found")
	}
	r.ensureDeps(pythonBin)
	inputPath := strings.ReplaceAll(inputFile, `\`, `/`)
	tag := r.model.Tag
	script := fmt.Sprintf(`import sys, os
os.environ["PYTHONIOENCODING"] = "utf-8"
try:
    from faster_whisper import WhisperModel as _FW
    _lib = "faster-whisper"
except ImportError:
    try:
        import whisper as _W
        _lib = "openai-whisper"
    except Exception:
        print("ERRORE: pip install faster-whisper")
        sys.exit(1)
_tag_map = {"large":"large-v3","medium":"medium","small":"small","base":"base","tiny":"tiny","turbo":"large-v3-turbo"}
_fw_model = _tag_map.get("%s", "large-v3")
if _lib == "faster-whisper":
    _m = _FW(_fw_model, device="cpu", compute_type="int8")
    _segs, _ = _m.transcribe("""%s""", task="translate", beam_size=5)
    print("".join(seg.text for seg in _segs))
else:
    _m = _W.load_model(_fw_model, device="cpu")
    print(_W.transcribe(_m, """%s""", task="translate")["text"])
`, tag, inputPath, inputPath)
	tmp, err := os.CreateTemp("", "vortelio-whisper-tr-*.py")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(script)
	tmp.Close()
	cmd := HideWindow(exec.Command(pythonBin, tmp.Name()))
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("translation failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SynthesizeToBytes runs TTS on text and returns the WAV bytes.
func (r *AudioRunner) SynthesizeToBytes(text string) ([]byte, error) {
	pythonBin := FindPython()
	if pythonBin == "" {
		return nil, fmt.Errorf("Python 3 not found")
	}
	r.ensureDeps(pythonBin)
	tmp, err := os.CreateTemp("", "vortelio-tts-*.wav")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)
	opts := &RunOptions{Prompt: text, OutputFile: tmpPath}
	var script string
	name := strings.ToLower(r.model.Name)
	switch {
	case strings.Contains(name, "bark"):
		script = r.buildBarkScript(opts)
	default:
		script = r.buildKokoroScript(opts)
	}
	if err := r.runPython(pythonBin, script); err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}
	return os.ReadFile(tmpPath)
}

func (r *AudioRunner) buildBarkScript(opts *RunOptions) string {
	text := opts.Prompt
	if text == "" { text = "Ciao." }
	outputPath := opts.OutputFile
	if outputPath == "" { outputPath = ResolveOutputPath("", "output.wav") }
	outputPath = strings.ReplaceAll(outputPath, `\`, `/`)
	device := r.deviceString(opts.ForceCPU)
	return fmt.Sprintf(`import sys, os
os.environ["PYTHONIOENCODING"] = "utf-8"
os.environ['SUNO_OFFLOAD_CPU'] = '1' if '%s' == 'cpu' else '0'
try:
    import torch as _torch
    # PyTorch 2.6+ compat: Bark uses weights_only=False semantics
    _orig_torch_load = _torch.load
    def _bark_torch_load(*a, **kw):
        kw.setdefault('weights_only', False)
        return _orig_torch_load(*a, **kw)
    _torch.load = _bark_torch_load
    from bark import SAMPLE_RATE, generate_audio, preload_models
    import scipy.io.wavfile as wav
    import numpy as np
except ImportError:
    print('Bark non installato. pip install git+https://github.com/suno-ai/bark.git scipy')
    sys.exit(1)
preload_models()
audio = generate_audio('''%s''')
wav.write(r'''%s''', SAMPLE_RATE, audio.astype(np.float32))
print('Audio salvato in: ' + r'''%s''')
`, device, escapePy(text), outputPath, outputPath)
}
