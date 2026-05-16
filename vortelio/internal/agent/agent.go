package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

var validAgentID = regexp.MustCompile(`^[a-z0-9_-]+$`)

// ── Catalog ───────────────────────────────────────────────────────────────────

type InstallMethod string

const (
	MethodNPM    InstallMethod = "npm"    // npm install -g <pkg>
	MethodPip    InstallMethod = "pip"    // pip install <pkg>
	MethodBinary InstallMethod = "binary" // direct exe download
)

type CatalogEntry struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	Version        string        `json:"version"`
	DefaultPort    int           `json:"default_port"`
	DefaultURL     string        `json:"default_url"`
	InstallMethod  InstallMethod `json:"install_method"`
	NPMPackage     string        `json:"npm_package,omitempty"`
	BinCommand     string        `json:"bin_command"`
	StartArgs      []string      `json:"start_args"`
	// EnvVars are extra environment variables injected at start time.
	// Use {{VORTELIO_URL}} as placeholder — replaced with the actual Vortelio base URL.
	EnvVars        []string      `json:"env_vars,omitempty"`
	HealthPath     string        `json:"health_path"`
	Tags           []string      `json:"tags"`
	RequiresAPIKey bool          `json:"requires_api_key"`
	APIKeyHint     string        `json:"api_key_hint,omitempty"`
}

// vortURL returns the base URL of the local Vortelio server.
func vortURL() string {
	return fmt.Sprintf("http://localhost:%d", config.Get().Port)
}

var Catalog = []CatalogEntry{
	{
		ID:          "openclaw",
		Name:        "OpenClaw",
		Description: "Gateway AI multi-canale (WhatsApp, Telegram, Discord). Usa i modelli locali di Vortelio.",
		Version:     "latest",
		DefaultPort: 18789,
		DefaultURL:  "http://localhost:18789",
		InstallMethod: MethodNPM,
		NPMPackage:  "openclaw",
		BinCommand:  "openclaw",
		StartArgs:   []string{"gateway", "--port", "18789"},
		// Point OpenClaw at Vortelio's Ollama-compatible API
		EnvVars: []string{
			"OLLAMA_HOST={{VORTELIO_URL}}",
			"OLLAMA_BASE_URL={{VORTELIO_URL}}",
			"OPENAI_BASE_URL={{VORTELIO_URL}}/v1",
			"OPENAI_API_BASE={{VORTELIO_URL}}/v1",
			"OPENAI_API_KEY=vortelio",
		},
		HealthPath:     "/",
		Tags:           []string{"chat", "gateway", "whatsapp", "telegram", "discord"},
		RequiresAPIKey: false,
	},
	{
		ID:          "opencode",
		Name:        "Open Code",
		Description: "Agente AI per sviluppatori: coding, refactoring, debug via terminale.",
		Version:     "latest",
		DefaultPort: 0, // TUI, no HTTP port
		DefaultURL:  "",
		InstallMethod: MethodNPM,
		NPMPackage:  "opencode-ai",
		BinCommand:  "opencode",
		StartArgs:   []string{},
		EnvVars: []string{
			"OPENAI_BASE_URL={{VORTELIO_URL}}/v1",
			"OPENAI_API_BASE={{VORTELIO_URL}}/v1",
			"OPENAI_API_KEY=vortelio",
		},
		HealthPath:     "",
		Tags:           []string{"coding", "git", "debug", "refactor"},
		RequiresAPIKey: false,
	},
	{
		ID:          "open-webui",
		Name:        "Open WebUI",
		Description: "Interfaccia web completa per chat con modelli locali. Compatibile Ollama. Richiede Python 3.11+.",
		Version:     "latest",
		DefaultPort: 3000,
		DefaultURL:  "http://localhost:3000",
		InstallMethod: MethodPip,
		NPMPackage:  "open-webui",
		BinCommand:  "open-webui",
		StartArgs:   []string{"serve", "--host", "0.0.0.0", "--port", "3000"},
		EnvVars: []string{
			"OLLAMA_BASE_URL={{VORTELIO_URL}}",
			"OPENAI_API_BASE_URL={{VORTELIO_URL}}/v1",
			"OPENAI_API_KEY=vortelio",
		},
		HealthPath:     "/health",
		Tags:           []string{"chat", "web", "ui", "ollama"},
		RequiresAPIKey: false,
	},
	{
		ID:          "flowise",
		Name:        "Flowise",
		Description: "Visual AI flow builder: crea agenti, RAG e workflow con drag-and-drop. Porta 3002.",
		Version:     "latest",
		DefaultPort: 3002,
		DefaultURL:  "http://localhost:3002",
		InstallMethod: MethodNPM,
		NPMPackage:  "flowise",
		BinCommand:  "flowise",
		StartArgs:   []string{"start"},
		EnvVars: []string{
			"PORT=3002",
			"FLOWISE_PORT=3002",
			"OPENAI_API_KEY=vortelio",
			"OPENAI_API_BASE={{VORTELIO_URL}}/v1",
		},
		HealthPath:     "/api/v1/ping",
		Tags:           []string{"flow", "agenti", "rag", "visual"},
		RequiresAPIKey: false,
	},
}

// ── State ─────────────────────────────────────────────────────────────────────

type AgentState struct {
	ID          string `json:"id"`
	Installed   bool   `json:"installed"`
	Running     bool   `json:"running"`
	PID         int    `json:"pid,omitempty"`
	Port        int    `json:"port"`
	NodeFound   bool   `json:"node_found"`
	Error       string `json:"error,omitempty"`
}

var (
	mu      sync.Mutex
	procMap = map[string]*exec.Cmd{}
)

// nodeAvailable checks if node and npm are in PATH.
func nodeAvailable() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

func npmAvailable() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// isBinInstalled checks if the agent binary is available.
func isBinInstalled(entry CatalogEntry) bool {
	if entry.InstallMethod == MethodPip {
		_, err := exec.LookPath(entry.BinCommand)
		if err == nil {
			return true
		}
		if runtime.GOOS == "windows" {
			_, err = exec.LookPath(entry.BinCommand + ".exe")
			return err == nil
		}
		return false
	}
	if entry.InstallMethod != MethodNPM {
		return false
	}
	// On Windows npm commands need .cmd suffix
	npmCmd := "npm"
	if runtime.GOOS == "windows" {
		npmCmd = "npm.cmd"
	}
	// Primary: npm list -g --depth=0 (most reliable)
	cmd := exec.Command(npmCmd, "list", "-g", "--depth=0", "--parseable")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			base := filepath.Base(strings.TrimSpace(line))
			if strings.EqualFold(base, entry.NPMPackage) ||
				strings.HasSuffix(strings.ToLower(line), "/"+strings.ToLower(entry.NPMPackage)) ||
				strings.HasSuffix(strings.ToLower(line), "\\"+strings.ToLower(entry.NPMPackage)) {
				return true
			}
		}
	}
	// Secondary: check the binary exists in npm global bin (LookPath is sufficient).
	binToCheck := entry.BinCommand
	if runtime.GOOS == "windows" {
		binToCheck = entry.BinCommand + ".cmd"
	}
	_, pathErr := exec.LookPath(binToCheck)
	return pathErr == nil
}

// GetState returns the current state of an agent.
func GetState(id string) AgentState {
	entry, ok := findEntry(id)
	if !ok {
		return AgentState{ID: id, Error: "agente sconosciuto"}
	}
	installed := isBinInstalled(entry)

	mu.Lock()
	cmd, running := procMap[id]
	mu.Unlock()

	pid := 0
	if running && cmd != nil && cmd.Process != nil {
		pid = cmd.Process.Pid
	}

	return AgentState{
		ID:        id,
		Installed: installed,
		Running:   running,
		PID:       pid,
		Port:      entry.DefaultPort,
		NodeFound: nodeAvailable(),
	}
}

// GetAllStates returns states for all catalog entries.
func GetAllStates() []AgentState {
	states := make([]AgentState, len(Catalog))
	for i, e := range Catalog {
		states[i] = GetState(e.ID)
	}
	return states
}

// ── Install ───────────────────────────────────────────────────────────────────

// Install installs the agent. For npm packages, streams npm output as progress.
func Install(ctx context.Context, id string, progress func(line string)) error {
	if !validAgentID.MatchString(id) {
		return fmt.Errorf("id agente non valido: %q", id)
	}
	entry, ok := findEntry(id)
	if !ok {
		return fmt.Errorf("agente sconosciuto: %s", id)
	}

	switch entry.InstallMethod {
	case MethodNPM:
		return installNPM(ctx, entry, progress)
	case MethodPip:
		return installPip(ctx, entry, progress)
	case MethodBinary:
		return fmt.Errorf("installazione binaria non supportata per questo agente")
	default:
		return fmt.Errorf("metodo di installazione sconosciuto")
	}
}

func installNPM(ctx context.Context, entry CatalogEntry, progress func(line string)) error {
	if !nodeAvailable() {
		return fmt.Errorf(
			"Node.js non trovato nel PATH.\n" +
			"Installa Node.js da https://nodejs.org (versione LTS consigliata),\n" +
			"poi riavvia Vortelio e riprova.",
		)
	}
	if !npmAvailable() {
		return fmt.Errorf(
			"npm non trovato nel PATH.\n" +
			"Assicurati che npm sia installato insieme a Node.js:\n" +
			"https://nodejs.org",
		)
	}

	npmCmd := "npm"
	if runtime.GOOS == "windows" {
		npmCmd = "npm.cmd"
	}

	args := []string{"install", "-g", entry.NPMPackage, "--prefer-online"}
	cmd := exec.CommandContext(ctx, npmCmd, args...)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("impossibile avviare npm: %w", err)
	}

	// Stream both stdout and stderr as progress lines
	done := make(chan struct{}, 2)
	streamLines := func(r io.Reader) {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 4096)
		var partial string
		for {
			n, err := r.Read(buf)
			if n > 0 {
				chunk := partial + string(buf[:n])
				lines := strings.Split(chunk, "\n")
				partial = lines[len(lines)-1]
				for _, l := range lines[:len(lines)-1] {
					l = strings.TrimSpace(l)
					if l != "" && progress != nil {
						progress(l)
					}
				}
			}
			if err != nil {
				break
			}
		}
	}
	go streamLines(stdout)
	go streamLines(stderr)
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("npm install fallito: %w\n\nSe il problema persiste, esegui manualmente:\n  npm install -g %s", err, entry.NPMPackage)
	}
	return nil
}

func installPip(ctx context.Context, entry CatalogEntry, progress func(line string)) error {
	pipCmd := "pip"
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath("pip3"); err == nil {
			pipCmd = "pip3"
		}
	} else {
		if _, err := exec.LookPath("pip3"); err == nil {
			pipCmd = "pip3"
		}
	}
	if _, err := exec.LookPath(pipCmd); err != nil {
		return fmt.Errorf(
			"pip non trovato nel PATH.\n" +
				"Installa Python 3.11+ da https://python.org,\n" +
				"poi riavvia Vortelio e riprova.",
		)
	}

	args := []string{"install", "--upgrade", entry.NPMPackage}
	cmd := exec.CommandContext(ctx, pipCmd, args...)

	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("impossibile avviare pip: %w", err)
	}

	done := make(chan struct{}, 2)
	streamLines := func(r io.Reader) {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 4096)
		var partial string
		for {
			n, err := r.Read(buf)
			if n > 0 {
				chunk := partial + string(buf[:n])
				lines := strings.Split(chunk, "\n")
				partial = lines[len(lines)-1]
				for _, l := range lines[:len(lines)-1] {
					l = strings.TrimSpace(l)
					if l != "" && progress != nil {
						progress(l)
					}
				}
			}
			if err != nil {
				break
			}
		}
	}
	go streamLines(stdout)
	go streamLines(stderr)
	<-done
	<-done

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("pip install fallito: %w\n\nSe il problema persiste, esegui manualmente:\n  pip install %s", err, entry.NPMPackage)
	}
	return nil
}

// ── Start ─────────────────────────────────────────────────────────────────────

func Start(id string) error {
	if !validAgentID.MatchString(id) {
		return fmt.Errorf("id agente non valido: %q", id)
	}
	entry, ok := findEntry(id)
	if !ok {
		return fmt.Errorf("agente sconosciuto: %s", id)
	}

	mu.Lock()
	if _, already := procMap[id]; already {
		mu.Unlock()
		return nil
	}
	mu.Unlock()

	if !isBinInstalled(entry) {
		return fmt.Errorf("agente %q non installato — installa prima tramite npm", entry.Name)
	}

	args := append([]string{}, entry.StartArgs...)
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmdArgs := append([]string{"/c", entry.BinCommand + ".cmd"}, args...)
		cmd = exec.Command("cmd", cmdArgs...)
	} else {
		cmd = exec.Command(entry.BinCommand, args...)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Build env: inherit system env + inject agent-specific vars
	env := os.Environ()
	for _, kv := range entry.EnvVars {
		resolved := strings.ReplaceAll(kv, "{{VORTELIO_URL}}", vortURL())
		env = append(env, resolved)
	}
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("avvio %s fallito: %w", entry.Name, err)
	}

	mu.Lock()
	procMap[id] = cmd
	mu.Unlock()

	go func() {
		cmd.Wait()
		mu.Lock()
		delete(procMap, id)
		mu.Unlock()
	}()

	return nil
}

// Stop terminates the agent process.
func Stop(id string) error {
	mu.Lock()
	cmd, ok := procMap[id]
	mu.Unlock()
	if !ok {
		return nil
	}
	if cmd != nil && cmd.Process != nil {
		cmd.Process.Kill()
	}
	mu.Lock()
	delete(procMap, id)
	mu.Unlock()
	return nil
}

// Uninstall removes the npm global package.
func Uninstall(id string) error {
	if !validAgentID.MatchString(id) {
		return fmt.Errorf("id agente non valido: %q", id)
	}
	Stop(id)
	entry, ok := findEntry(id)
	if !ok {
		return fmt.Errorf("agente sconosciuto")
	}
	switch entry.InstallMethod {
	case MethodNPM:
		npmCmd := "npm"
		if runtime.GOOS == "windows" {
			npmCmd = "npm.cmd"
		}
		return exec.Command(npmCmd, "uninstall", "-g", entry.NPMPackage).Run()
	case MethodPip:
		pipCmd := "pip"
		if _, err := exec.LookPath("pip3"); err == nil {
			pipCmd = "pip3"
		}
		return exec.Command(pipCmd, "uninstall", "-y", entry.NPMPackage).Run()
	default:
		return fmt.Errorf("rimozione non supportata per questo tipo di agente")
	}
}

// Health checks if the agent HTTP server is responding.
func Health(id string) (bool, string) {
	entry, ok := findEntry(id)
	if !ok {
		return false, "agente sconosciuto"
	}
	if entry.DefaultURL == "" {
		// TUI agent — check if process is alive
		mu.Lock()
		_, alive := procMap[id]
		mu.Unlock()
		if alive {
			return true, "process running (no HTTP endpoint)"
		}
		return false, "not running"
	}
	url := entry.DefaultURL + entry.HealthPath
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	return resp.StatusCode < 400, string(body)
}

// AgentBinDir returns the agents directory inside VORTELIO_HOME.
func AgentBinDir() string {
	return filepath.Join(config.HomeDir(), "agents")
}

// ── JSON catalog ──────────────────────────────────────────────────────────────

func CatalogJSON() []byte {
	type entry struct {
		CatalogEntry
		State AgentState `json:"state"`
	}
	out := make([]entry, len(Catalog))
	for i, e := range Catalog {
		out[i] = entry{CatalogEntry: e, State: GetState(e.ID)}
	}
	b, _ := json.Marshal(out)
	return b
}

func findEntry(id string) (CatalogEntry, bool) {
	for _, e := range Catalog {
		if e.ID == id {
			return e, true
		}
	}
	return CatalogEntry{}, false
}
