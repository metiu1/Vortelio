package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	rt "github.com/vortelio/vortelio/internal/runtime"
)

// handleRunCode executes a code snippet locally and returns its output. This is a
// developer convenience (the user explicitly asked to run code in Vortelio); it
// runs on the user's own machine with a timeout. POST {language, code}.
func handleRunCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Language string `json:"language"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON")
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		jsonError(w, 400, "code is required")
		return
	}

	lang := strings.ToLower(strings.TrimSpace(req.Language))
	dir, err := os.MkdirTemp("", "vortelio-run-*")
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer os.RemoveAll(dir)

	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	write := func(name string) string {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(req.Code), 0644)
		return p
	}

	switch lang {
	case "python", "py", "python3":
		py := rt.FindPython()
		if py == "" {
			jsonError(w, 400, "Python non trovato sul sistema")
			return
		}
		cmd = exec.CommandContext(ctx, py, write("snippet.py"))
	case "javascript", "js", "node", "nodejs":
		node := findExec("node", "node.exe")
		if node == "" {
			jsonError(w, 400, "Node.js non trovato sul sistema")
			return
		}
		cmd = exec.CommandContext(ctx, node, write("snippet.js"))
	case "bash", "sh", "shell":
		if runtime.GOOS == "windows" {
			if bash := findExec("bash", "bash.exe"); bash != "" {
				cmd = exec.CommandContext(ctx, bash, write("snippet.sh"))
			} else {
				cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", req.Code)
			}
		} else {
			cmd = exec.CommandContext(ctx, "sh", write("snippet.sh"))
		}
	case "powershell", "ps", "ps1":
		cmd = exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", req.Code)
	case "go", "golang":
		if findExec("go", "go.exe") == "" {
			jsonError(w, 400, "Go non trovato sul sistema")
			return
		}
		cmd = exec.CommandContext(ctx, "go", "run", write("snippet.go"))
	default:
		jsonError(w, 400, "Linguaggio non eseguibile: "+lang+" (supportati: python, javascript, bash, powershell, go)")
		return
	}

	cmd.Dir = dir
	cmd = rt.HideWindow(cmd)
	out, runErr := cmd.CombinedOutput()
	output := string(out)
	if len(output) > 100*1024 {
		output = output[:100*1024] + "\n…[output troncato]"
	}
	resp := map[string]interface{}{"output": output, "ok": runErr == nil}
	if ctx.Err() == context.DeadlineExceeded {
		resp["ok"] = false
		resp["output"] = output + "\n…[interrotto dopo 30s]"
	} else if runErr != nil {
		resp["error"] = runErr.Error()
	}
	respond(w, 200, resp)
}

// findExec resolves the first of the given names found in PATH.
func findExec(names ...string) string {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p
		}
	}
	return ""
}
