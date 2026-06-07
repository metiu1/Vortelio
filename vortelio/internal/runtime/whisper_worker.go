package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// A persistent Whisper worker keeps the model loaded in a long-running Python
// process and transcribes audio paths fed over stdin. This removes the multi-
// second model-reload cost that made one-shot transcription feel very slow.

type whisperWorker struct {
	tag    string
	proc   *procHandle
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
}

type procHandle struct {
	kill func() error
}

var (
	wWorker   *whisperWorker
	wWorkerMu sync.Mutex
)

const whisperWorkerScript = `import sys, json
try:
    from faster_whisper import WhisperModel
except Exception:
    print(json.dumps({"error": "faster-whisper non installato"}), flush=True)
    sys.exit(1)
_tag = sys.argv[1] if len(sys.argv) > 1 else "base"
_m = {"large":"large-v3","large-v3":"large-v3","large-v2":"large-v2","medium":"medium",
      "small":"small","base":"base","tiny":"tiny","turbo":"large-v3-turbo"}
import re as _re
_name = _m.get(_re.split(r"[/\\\\]", _tag)[-1].lower().strip(), "base")
# Try GPU first (much faster); GTX 16xx/Turing don't do efficient fp16, so use
# int8_float16/int8 on CUDA, then fall back to CPU int8.
model = None
_dev = _ct = None
_last = ""
for _d, _c in (("cuda", "int8_float16"), ("cuda", "int8"), ("cpu", "int8")):
    try:
        model = WhisperModel(_name, device=_d, compute_type=_c)
        _dev, _ct = _d, _c
        break
    except Exception as _e:
        _last = str(_e)
if model is None:
    print(json.dumps({"error": "load: " + _last}), flush=True)
    sys.exit(1)
print(json.dumps({"ready": True, "device": _dev, "compute": _ct}), flush=True)
for line in sys.stdin:
    path = line.strip().strip('"')
    if not path:
        continue
    try:
        segs, _info = model.transcribe(path, beam_size=1)
        text = "".join(s.text for s in segs).strip()
        print(json.dumps({"text": text}), flush=True)
    except Exception as e:
        print(json.dumps({"error": str(e)}), flush=True)
`

// TranscribeViaWorker transcribes audioPath using a persistent Whisper worker for
// the given model tag (e.g. "base"). Falls back to an error if Python/worker is
// unavailable; the caller can then use the one-shot path.
func TranscribeViaWorker(modelTag, audioPath string) (string, error) {
	wWorkerMu.Lock()
	if wWorker == nil || wWorker.tag != modelTag {
		if wWorker != nil {
			wWorker.close()
		}
		w, err := startWhisperWorker(modelTag)
		if err != nil {
			wWorkerMu.Unlock()
			return "", err
		}
		wWorker = w
	}
	w := wWorker
	wWorkerMu.Unlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := io.WriteString(w.stdin, audioPath+"\n"); err != nil {
		dropWorker(w)
		return "", err
	}
	line, err := w.stdout.ReadString('\n')
	if err != nil {
		dropWorker(w)
		return "", fmt.Errorf("worker non risponde: %w", err)
	}
	var res struct {
		Text  string `json:"text"`
		Error string `json:"error"`
	}
	if e := json.Unmarshal([]byte(strings.TrimSpace(line)), &res); e != nil {
		return "", fmt.Errorf("output worker non valido: %s", strings.TrimSpace(line))
	}
	if res.Error != "" {
		return "", fmt.Errorf("%s", res.Error)
	}
	return res.Text, nil
}

func dropWorker(w *whisperWorker) {
	wWorkerMu.Lock()
	if wWorker == w {
		wWorker = nil
	}
	wWorkerMu.Unlock()
	w.close()
}

func (w *whisperWorker) close() {
	if w.stdin != nil {
		w.stdin.Close()
	}
	if w.proc != nil && w.proc.kill != nil {
		w.proc.kill()
	}
}

func startWhisperWorker(tag string) (*whisperWorker, error) {
	py := FindPython()
	if py == "" {
		return nil, fmt.Errorf("Python non trovato")
	}
	tmp, err := os.CreateTemp("", "vortelio-whisper-worker-*.py")
	if err != nil {
		return nil, err
	}
	tmp.WriteString(whisperWorkerScript)
	tmp.Close()

	cmd := HideWindow(exec.Command(py, "-u", tmp.Name(), tag))
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() { _ = cmd.Wait(); os.Remove(tmp.Name()) }()

	w := &whisperWorker{
		tag:    tag,
		stdin:  stdin,
		stdout: bufio.NewReader(stdoutPipe),
		proc:   &procHandle{kill: func() error { return cmd.Process.Kill() }},
	}

	// Wait for the ready (or error) line, with a generous load timeout.
	type lineRes struct {
		line string
		err  error
	}
	ch := make(chan lineRes, 1)
	go func() { l, e := w.stdout.ReadString('\n'); ch <- lineRes{l, e} }()
	select {
	case r := <-ch:
		if r.err != nil {
			w.close()
			return nil, fmt.Errorf("avvio worker fallito: %w", r.err)
		}
		var got struct {
			Ready   bool   `json:"ready"`
			Error   string `json:"error"`
			Device  string `json:"device"`
			Compute string `json:"compute"`
		}
		json.Unmarshal([]byte(strings.TrimSpace(r.line)), &got)
		if got.Error != "" {
			w.close()
			return nil, fmt.Errorf("%s", got.Error)
		}
		fmt.Printf("🎤  Whisper worker pronto (%s · %s · %s)\n", tag, got.Device, got.Compute)
		return w, nil
	case <-time.After(180 * time.Second):
		w.close()
		return nil, fmt.Errorf("timeout caricamento modello Whisper")
	}
}
