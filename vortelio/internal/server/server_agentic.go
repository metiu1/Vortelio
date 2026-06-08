package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/mcp"
	rt "github.com/vortelio/vortelio/internal/runtime"
)

// ── Approval broker ────────────────────────────────────────────────────────────
//
// Risky coding tool calls (shell, write, edit) block on a decision delivered out
// of band by POST /api/agentic/approve. The streaming chat connection emits an
// "approval_request" tool event; the UI shows approve/deny and resolves it.

type approvalReq struct {
	ch        chan bool
	createdAt time.Time
}

var (
	approvalsMu sync.Mutex
	approvals   = map[string]*approvalReq{}
)

func registerApproval(id string) chan bool {
	ch := make(chan bool, 1)
	approvalsMu.Lock()
	approvals[id] = &approvalReq{ch: ch, createdAt: time.Now()}
	approvalsMu.Unlock()
	return ch
}

func resolveApproval(id string, ok bool) bool {
	approvalsMu.Lock()
	a, found := approvals[id]
	if found {
		delete(approvals, id)
	}
	approvalsMu.Unlock()
	if !found {
		return false
	}
	a.ch <- ok
	return true
}

// ── ask_user (interactive question with options) ────────────────────
var (
	asksMu sync.Mutex
	asks   = map[string]chan string{}
)

func registerAsk(id string) chan string {
	ch := make(chan string, 1)
	asksMu.Lock()
	asks[id] = ch
	asksMu.Unlock()
	return ch
}

func resolveAsk(id, answer string) bool {
	asksMu.Lock()
	ch, ok := asks[id]
	if ok {
		delete(asks, id)
	}
	asksMu.Unlock()
	if !ok {
		return false
	}
	ch <- answer
	return true
}

// POST /api/agentic/answer  — {id, answer}
func handleAgenticAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "use POST")
		return
	}
	var req struct {
		ID     string `json:"id"`
		Answer string `json:"answer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON")
		return
	}
	if !resolveAsk(req.ID, req.Answer) {
		jsonError(w, 404, "no pending question with that id")
		return
	}
	respond(w, 200, map[string]string{"status": "ok"})
}

// POST /api/agentic/approve  — {id, approved}
func handleAgenticApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "use POST")
		return
	}
	var req struct {
		ID       string `json:"id"`
		Approved bool   `json:"approved"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if !resolveApproval(req.ID, req.Approved) {
		jsonError(w, 404, "no pending approval with that id (it may have timed out)")
		return
	}
	respond(w, 200, map[string]string{"status": "ok"})
}

// autoSystemPrompt prepends a short instruction telling the model it can call
// the available tools (web search, math, files, media generation) on its own.
// Used in smart/auto mode so a beginner never has to toggle anything.
func autoSystemPrompt(existing string) string {
	nudge := "You are a friendly, helpful assistant. Chat naturally with the user and answer in their own " +
		"language. You also have tools available (web search and image/audio/video/3D generation) that you may " +
		"use when the user clearly needs up-to-date information or asks you to create media. For greetings, " +
		"casual conversation, or anything you already know, just reply normally without using a tool. " +
		"After a tool returns, write a clear, complete answer to the user IN THEIR LANGUAGE using the results — " +
		"never just describe or repeat the raw JSON/output. " +
		"When a visual or interactive answer would help (a chart, diagram, calculator, table, mock UI, game, " +
		"animation…), you MAY reply with a single self-contained ```html code block (inline CSS/JS allowed) — it " +
		"will be rendered live for the user. Use this whenever building it answers the question better than text. " +
		conciseNudgeEN + " " +
		askUserNudgeEN
	if strings.TrimSpace(existing) == "" {
		return nudge
	}
	return nudge + "\n\n" + existing
}

// autonomousSystemPrompt drives a goal-seeking agent that keeps working across
// many tool rounds until the objective is fully achieved (e.g. building a whole
// project in Developer mode), without asking for confirmation at each step.
func autonomousSystemPrompt(existing string) string {
	nudge := "You are an AUTONOMOUS agent. The user gives you a GOAL; your job is to reach it on your own. " +
		"Work in a loop: (1) think briefly and break the goal into concrete steps; (2) use your tools " +
		"(read/list/glob/grep files, write_file/edit, run_shell, run code, web_search, media, create_skill) to " +
		"execute each step; (3) verify your work by reading files back and running it; (4) fix problems and " +
		"continue. Build complete, working projects: create every needed file with real content, wire them " +
		"together, and run them to confirm they work. Do NOT stop to ask for permission or confirmation — act, " +
		"and only pause if you truly cannot proceed. When you hit a reusable procedure worth keeping, call " +
		"create_skill to save it. Keep going until the goal is fully met, then end with a short summary of what " +
		"you built and how to use it. " +
		"Stay autonomous, but if you hit a genuinely ambiguous choice you cannot reasonably decide on your own, " +
		"use ask_user(question, 2-5 options) to get a quick graphical decision from the user, then continue. " +
		conciseNudgeEN
	if strings.TrimSpace(existing) == "" {
		return nudge
	}
	return nudge + "\n\n" + existing
}

// ── Agentic provider builder ───────────────────────────────────────────────────

// buildAgenticProvider assembles a composite tool provider from the request's
// AgenticConfig. emit is the per-request tool event emitter (used for approvals).
// BuildCodingHarness exposes the exact same agentic tool harness the Developer
// GUI uses (builtins + coding tools + web + self skills), for the Vortelio CLI.
func BuildCodingHarness(workingDir, mode string, autonomous bool, emit rt.ToolEventEmitter) rt.ToolProvider {
	cfg := &AgenticConfig{
		Auto:       true,
		Autonomous: autonomous,
		WebSearch:  true,
		Builtins:   true,
		Coding:     true,
		Mode:       mode,
		WorkingDir: workingDir,
	}
	return buildAgenticProvider(cfg, emit)
}

// CodingSystemPrompt returns the system prompt for the CLI coding agent, matching
// the GUI behaviour (autonomous goal-seeking when requested).
func CodingSystemPrompt(autonomous bool) string {
	if autonomous {
		return autonomousSystemPrompt("")
	}
	return "Sei Vortelio Code, un agente di coding che lavora nel terminale dentro la cartella di lavoro " +
		"descritta nel CONTESTO WORKSPACE. Rispondi nella lingua dell'utente. " +
		"Sei orientato al progetto corrente: le richieste dell'utente riguardano quasi sempre i file e il codice " +
		"di QUESTA cartella, non attività generiche. " +
		"Prima di rispondere su \"il progetto\", \"questo\", \"qui\" o un file citato, USA gli strumenti " +
		"(list_directory, read_file, glob_search, grep_search) per guardare i file reali: non indovinare e non inventare contenuti. " +
		"Per modificare il progetto usa write_file / edit_file con percorsi relativi alla cartella di lavoro e " +
		"riferisci sempre il percorso esatto. Non affermare di aver creato o cambiato un file se non hai chiamato lo strumento. " +
		"Hai anche strumenti web (web_search) e di generazione media (immagini/audio/video/3D): usali solo quando " +
		"l'utente li chiede davvero, e per impostazione predefinita salva gli artefatti dentro la cartella di lavoro. " +
		"Dopo che uno strumento restituisce un risultato, scrivi una risposta chiara e completa nella lingua dell'utente; " +
		"non limitarti a ripetere il JSON grezzo. " +
		conciseNudgeIT + " " +
		askUserNudgeIT
}

// conciseNudgeIT / conciseNudgeEN push the model to minimize output tokens:
// answer essentially, without filler, padding or redundant recap.
const conciseNudgeIT = "SII CONCISO e minimizza i token di output: vai dritto al punto, niente preamboli, " +
	"chiacchiere, ripetizioni o riassunti ridondanti. Rispondi solo a ciò che è stato chiesto, con il minor numero " +
	"di parole possibile; preferisci elenchi puntati brevi a lunghe tabelle e paragrafi, ometti dettagli ovvi o non " +
	"richiesti. Resta corretto e completo, ma essenziale: meno testo è meglio."

const conciseNudgeEN = "BE CONCISE and minimize output tokens: get straight to the point, no preamble, filler, " +
	"repetition or redundant recap. Answer only what was asked, in as few words as possible; prefer short bullet " +
	"lists over long tables and paragraphs, and omit obvious or unrequested detail. Stay correct and complete, but lean."

// askUserNudgeIT / askUserNudgeEN instruct the model to use the graphical
// ask_user tool instead of guessing when it needs a decision from the user.
const askUserNudgeIT = "Quando ti serve una decisione o un chiarimento dall'utente (più strade valide, dati mancanti, " +
	"conferma di un approccio), NON tirare a indovinare: chiama lo strumento ask_user con una domanda chiara e 2-5 " +
	"opzioni — all'utente comparirà un popup grafico con i pulsanti delle opzioni e un campo \"Altro\" per la risposta libera. Usalo solo quando serve davvero."

const askUserNudgeEN = "When you need a decision or clarification from the user (several valid options, missing info, " +
	"confirming an approach), call the ask_user tool with a clear question and 2-5 options instead of guessing — " +
	"the user gets a graphical popup with option buttons and an \"Other\" free-text field. Use it only when it genuinely helps."

// workspaceContext tells the agent which folder it is working in AND gives it a
// live snapshot of that folder (git branch, project type, file tree) so it knows
// "where it is" and what "this project" / "qui" refers to without having to guess.
func workspaceContext(cfg *AgenticConfig) string {
	if cfg == nil || !cfg.Coding || strings.TrimSpace(cfg.WorkingDir) == "" {
		return ""
	}
	dir := cfg.WorkingDir
	var b strings.Builder
	b.WriteString("=== CONTESTO WORKSPACE (aggiornato a questo turno) ===\n")
	b.WriteString("CARTELLA DI LAVORO (radice del progetto): " + dir + "\n")
	if branch, clean := workspaceGitInfo(dir); branch != "" {
		st := "modificato"
		if clean {
			st = "pulito"
		}
		b.WriteString("Git: branch " + branch + " (" + st + ")\n")
	}
	if kind := detectProjectKind(dir); kind != "" {
		b.WriteString("Tipo progetto: " + kind + "\n")
	}
	if tree := workspaceTree(dir); tree != "" {
		b.WriteString("Contenuto della cartella (i file REALI presenti qui ora):\n" + tree)
	}
	if sum := projectSummaryExcerpt(dir); sum != "" {
		b.WriteString("\nRIASSUNTO DEL PROGETTO (da PROJECT.md, generato da /init):\n" + sum + "\n")
		b.WriteString("Se durante il lavoro modifichi qualcosa che rende PROJECT.md obsoleto (nuovi file/moduli, comandi, dipendenze, configurazione), AGGIORNA PROJECT.md con write_file/edit_file per tenerlo allineato.\n")
	}
	b.WriteString("\nREGOLE DI CONTESTO:\n")
	b.WriteString("- Quando l'utente dice \"questo progetto\", \"qui\", \"il sistema\", \"questa cartella\" si riferisce SEMPRE alla cartella di lavoro qui sopra. Non chiedere di quale progetto si tratta: leggilo.\n")
	b.WriteString("- Prima di rispondere a domande sul progetto o di eseguire azioni su di esso, ISPEZIONA i file reali con list_directory / read_file invece di indovinare o inventare.\n")
	b.WriteString("- Se l'utente cita un percorso o un file (es. agent.py), aprilo con read_file prima di rispondere.\n")
	b.WriteString("- I percorsi relativi degli strumenti file sono relativi a questa cartella; usa write_file / edit_file per creare o modificare file e indica SEMPRE il percorso esatto.\n")
	b.WriteString("- Non dire mai che un file è stato creato o modificato se non hai davvero chiamato lo strumento corrispondente.\n")
	b.WriteString("- Non confondere la cartella di lavoro con cartelle temporanee di sistema: salva e riferisci i file dentro la cartella di lavoro salvo richiesta esplicita diversa.\n")
	return b.String()
}

// projectSummaryExcerpt returns the first lines of PROJECT.md if present, so the
// agent always has the project summary in context and can keep it up to date.
func projectSummaryExcerpt(dir string) string {
	data, err := os.ReadFile(filepath.Join(dir, "PROJECT.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	const maxLines = 40
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "… (PROJECT.md continua)")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// workspaceGitInfo returns the current branch and whether the tree is clean.
func workspaceGitInfo(dir string) (string, bool) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", false
	}
	branch := strings.TrimSpace(string(out))
	st, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	return branch, strings.TrimSpace(string(st)) == ""
}

// detectProjectKind guesses the project type from marker files in the root.
func detectProjectKind(dir string) string {
	markers := []struct{ file, kind string }{
		{"go.mod", "Go"},
		{"package.json", "Node.js / JavaScript"},
		{"pyproject.toml", "Python"},
		{"requirements.txt", "Python"},
		{"Cargo.toml", "Rust"},
		{"pom.xml", "Java (Maven)"},
		{"build.gradle", "Java/Kotlin (Gradle)"},
		{"CMakeLists.txt", "C/C++ (CMake)"},
	}
	var kinds []string
	seen := map[string]bool{}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m.file)); err == nil {
			if !seen[m.kind] {
				kinds = append(kinds, m.kind)
				seen[m.kind] = true
			}
		}
	}
	return strings.Join(kinds, ", ")
}

// workspaceTree returns a compact listing of the workspace: top-level entries
// plus one level of nesting, skipping noise dirs, capped so it never floods the
// prompt. This is what lets the model know what "this project" actually contains.
func workspaceTree(dir string) string {
	skip := map[string]bool{
		".git": true, "node_modules": true, ".venv": true, "venv": true,
		"__pycache__": true, "dist": true, "build": true, ".next": true,
		"target": true, ".idea": true, ".vscode": true, "vendor": true,
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	var b strings.Builder
	lines := 0
	const maxLines = 60
	for _, e := range entries {
		if lines >= maxLines {
			b.WriteString("  … (altri file omessi)\n")
			break
		}
		name := e.Name()
		if e.IsDir() {
			b.WriteString("  " + name + "/\n")
			lines++
			if skip[name] {
				continue
			}
			// one level of children
			children, err := os.ReadDir(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			sort.Slice(children, func(i, j int) bool {
				if children[i].IsDir() != children[j].IsDir() {
					return children[i].IsDir()
				}
				return children[i].Name() < children[j].Name()
			})
			shown := 0
			for _, c := range children {
				if lines >= maxLines {
					break
				}
				if shown >= 12 {
					b.WriteString("    … (" + name + " ha altri file)\n")
					lines++
					break
				}
				suffix := ""
				if c.IsDir() {
					suffix = "/"
				}
				b.WriteString("    " + c.Name() + suffix + "\n")
				lines++
				shown++
			}
		} else {
			b.WriteString("  " + name + "\n")
			lines++
		}
	}
	return b.String()
}

// SkillInfo is a lightweight skill descriptor for the CLI.
type SkillInfo struct {
	ID      string
	Name    string
	Builtin bool
}

// ListSkillInfos returns all available skills (builtin + custom) for the CLI.
func ListSkillInfos() []SkillInfo {
	out := []SkillInfo{}
	for _, s := range listSkills() {
		out = append(out, SkillInfo{ID: s.ID, Name: s.Name, Builtin: s.Builtin})
	}
	return out
}

// BuildCLIHarness builds the full agentic harness for the CLI with optional MCP
// and skills, and returns the matching system prompt (skills applied). approve is
// the synchronous approval callback used for "ask" mode in the terminal.
func BuildCLIHarness(workingDir, mode string, autonomous, mcpOn bool, skills []string, emit rt.ToolEventEmitter, approve func(tool, summary, args string) bool, ask func(question string, options []string) string) (rt.ToolProvider, string) {
	cfg := &AgenticConfig{
		Auto:        true,
		Autonomous:  autonomous,
		WebSearch:   true,
		Builtins:    true,
		Coding:      true,
		Media:       true,
		MCP:         mcpOn,
		Mode:        mode,
		WorkingDir:  workingDir,
		Skills:      skills,
		ApproveFunc: approve,
		AskFunc:     ask,
	}
	sys := CodingSystemPrompt(autonomous)
	if len(skills) > 0 {
		sys = applySkills(sys, skills)
	}
	if ws := workspaceContext(cfg); ws != "" {
		sys = ws + "\n\n" + sys
	}
	return buildAgenticProvider(cfg, emit), sys
}

func buildAgenticProvider(cfg *AgenticConfig, emit rt.ToolEventEmitter) rt.ToolProvider {
	var providers []rt.ToolProvider

	if cfg.WebSearch || cfg.Builtins {
		providers = append(providers, &filteredBuiltins{web: cfg.WebSearch, rest: cfg.Builtins})
	}
	if cfg.MCP {
		providers = append(providers, mcp.Default().Provider())
	}
	if cfg.Coding {
		providers = append(providers, newCodingProvider(cfg, emit))
	}
	if cfg.Media {
		providers = append(providers, newMediaProvider(emit))
	}
	// The agent can author its own skills and ask the user interactive questions.
	if len(providers) > 0 {
		providers = append(providers, &selfProvider{emit: emit, ask: cfg.AskFunc})
	}
	mode := cfg.Mode
	if mode == "" {
		mode = "ask"
	}
	// Centralized approval gate: in Ask mode every state-changing/code-running
	// tool (run_code, media generation, install, rename, create_document) must be
	// confirmed; in Plan mode they are blocked. (write_file/edit_file/run_shell
	// are gated inside the coding provider itself, so they are not listed here to
	// avoid a double prompt.)
	return &gatedProvider{
		inner:   rt.NewCompositeProvider(providers...),
		mode:    mode,
		approve: cfg.ApproveFunc,
		emit:    emit,
	}
}

// gatedRiskyTools are tools that act on the system or run code and therefore need
// confirmation in Ask mode (and are blocked in Plan mode).
var gatedRiskyTools = map[string]bool{
	"run_code":        true,
	"create_document": true,
	"generate_image":  true,
	"generate_video":  true,
	"text_to_speech":  true,
	"generate_3d":     true,
	"install_model":   true,
	"rename_file":     true,
}

// gatedProvider wraps the full tool set and enforces Ask/Plan-mode approval for
// risky tools that the per-tool providers do not gate themselves.
type gatedProvider struct {
	inner   rt.ToolProvider
	mode    string
	approve func(tool, summary, args string) bool
	emit    rt.ToolEventEmitter
	mu      sync.Mutex
	counter int
}

func (g *gatedProvider) Tools() []rt.ToolDef { return g.inner.Tools() }

func (g *gatedProvider) Execute(name, args string) (string, error) {
	if gatedRiskyTools[name] {
		switch g.mode {
		case "plan":
			return "", fmt.Errorf("blocked: in Plan mode the agent cannot run code or generate/modify files. Switch to Ask or Auto to proceed")
		case "auto":
			// proceed without prompting
		default: // ask
			if !g.requestApproval(name, gatedSummary(name, args), args) {
				return "", fmt.Errorf("denied by user")
			}
		}
	}
	return g.inner.Execute(name, args)
}

func gatedSummary(name, args string) string {
	var m map[string]interface{}
	json.Unmarshal([]byte(args), &m)
	clip := func(s string, n int) string {
		s = strings.ReplaceAll(s, "\n", " ")
		if len(s) > n {
			return s[:n] + "…"
		}
		return s
	}
	switch name {
	case "run_code":
		return "Esegui codice: " + clip(fmt.Sprint(m["code"]), 120)
	case "create_document":
		return fmt.Sprintf("Crea documento: %v", m["path"])
	case "install_model":
		return fmt.Sprintf("Installa modello: %v", m["model"])
	case "rename_file":
		return fmt.Sprintf("Rinomina file: %v", m["path"])
	case "generate_image", "generate_video", "generate_3d", "text_to_speech":
		return fmt.Sprintf("%s: %v", name, clip(fmt.Sprint(m["prompt"]), 100))
	}
	return name
}

func (g *gatedProvider) requestApproval(tool, summary, argsJSON string) bool {
	if g.approve != nil { // CLI synchronous y/n
		return g.approve(tool, summary, argsJSON)
	}
	g.mu.Lock()
	g.counter++
	id := fmt.Sprintf("gappr_%d_%d", time.Now().UnixNano(), g.counter)
	g.mu.Unlock()
	ch := registerApproval(id)
	if g.emit != nil {
		g.emit("approval_request", map[string]interface{}{
			"id": id, "tool": tool, "summary": summary, "arguments": json.RawMessage(argsJSON),
		})
	}
	select {
	case ok := <-ch:
		return ok
	case <-time.After(5 * time.Minute):
		resolveApproval(id, false)
		return false
	}
}

// isNoiseDir reports directories whose contents are build artefacts or VCS state
// and should be skipped by grep/glob so the model isn't fed binary garbage.
func isNoiseDir(name string) bool {
	switch name {
	case ".git", "__pycache__", "node_modules", ".venv", "venv", "dist", "build", ".idea", ".vscode", ".mypy_cache", ".pytest_cache", ".next", "target":
		return true
	}
	return false
}

// isBinaryName reports paths with a non-text extension that grep/glob should skip.
func isBinaryName(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pyc", ".pyo", ".exe", ".dll", ".so", ".dylib", ".o", ".a", ".class", ".jar",
		".zip", ".gz", ".tar", ".7z", ".rar", ".bin", ".db", ".sqlite", ".sqlite3",
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".ico", ".tiff",
		".mp4", ".mov", ".avi", ".mkv", ".webm", ".mp3", ".wav", ".flac", ".ogg",
		".pdf", ".woff", ".woff2", ".ttf", ".otf", ".eot":
		return true
	}
	return false
}

// selfProvider lets the agent create reusable skills and ask the user questions
// with a graphical option picker.
type selfProvider struct {
	emit    rt.ToolEventEmitter
	ask     func(question string, options []string) string // CLI synchronous prompt; nil = GUI popup
	counter int
}

func (s *selfProvider) Tools() []rt.ToolDef {
	return []rt.ToolDef{
		{Type: "function", Function: rt.ToolFuncDef{
			Name:        "create_skill",
			Description: "Save a reusable skill to the user's skill library so it can be enabled in future sessions. Use when you find a repeatable procedure, style, or instruction set worth keeping.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"Short skill name, e.g. 'React component author'"},"description":{"type":"string","description":"One-line summary of what the skill does"},"body":{"type":"string","description":"The full instructions the model should follow when this skill is active"}},"required":["name","body"]}`),
		}},
		{Type: "function", Function: rt.ToolFuncDef{
			Name:        "ask_user",
			Description: "Ask the user a question and let them choose from options in a graphical popup (with a free-text 'Other' field). Use when you need a decision or clarification before continuing. Returns the user's answer.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"question":{"type":"string","description":"The question to ask the user"},"options":{"type":"array","items":{"type":"string"},"description":"2-5 suggested answers shown as buttons"}},"required":["question"]}`),
		}},
	}
}

func (s *selfProvider) Execute(name, args string) (string, error) {
	switch name {
	case "create_skill":
		var a struct {
			Name, Description, Body string
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		if strings.TrimSpace(a.Name) == "" || strings.TrimSpace(a.Body) == "" {
			return "", fmt.Errorf("name and body are required")
		}
		id, err := saveSkillContent("", a.Name, a.Description, a.Body)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Skill \"%s\" created (id: %s).", a.Name, id), nil
	case "ask_user":
		var a struct {
			Question string   `json:"question"`
			Options  []string `json:"options"`
		}
		if err := json.Unmarshal([]byte(args), &a); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		if strings.TrimSpace(a.Question) == "" {
			return "", fmt.Errorf("question is required")
		}
		// CLI: synchronous terminal prompt.
		if s.ask != nil {
			return s.ask(a.Question, a.Options), nil
		}
		// GUI: emit a popup event and block until the user answers.
		s.counter++
		id := fmt.Sprintf("ask_%d_%d", time.Now().UnixNano(), s.counter)
		ch := registerAsk(id)
		if s.emit != nil {
			s.emit("ask_user", map[string]interface{}{"id": id, "question": a.Question, "options": a.Options})
		}
		select {
		case ans := <-ch:
			return "L'utente ha risposto: " + ans, nil
		case <-time.After(10 * time.Minute):
			resolveAsk(id, "")
			return "", fmt.Errorf("nessuna risposta dall'utente (timeout)")
		}
	}
	return "", fmt.Errorf("unknown tool: %s", name)
}

// filteredBuiltins exposes the builtin tools, optionally limited to web_search.
type filteredBuiltins struct {
	web  bool
	rest bool
}

func (f *filteredBuiltins) Tools() []rt.ToolDef {
	var out []rt.ToolDef
	for _, t := range rt.BuiltinTools() {
		name := t.Function.Name
		if name == "web_search" {
			if f.web {
				out = append(out, t)
			}
			continue
		}
		if f.rest {
			out = append(out, t)
		}
	}
	return out
}

func (f *filteredBuiltins) Execute(name, args string) (string, error) {
	return rt.ExecuteTool(name, args)
}

// ── Coding tool provider ───────────────────────────────────────────────────────

type codingProvider struct {
	mode    string // "plan" | "ask" | "auto"
	root    string
	emit    rt.ToolEventEmitter
	approve func(tool, summary, args string) bool // synchronous approval (CLI); nil = use HTTP flow
	counter int
	mu      sync.Mutex
}

func newCodingProvider(cfg *AgenticConfig, emit rt.ToolEventEmitter) *codingProvider {
	mode := cfg.Mode
	if mode == "" {
		mode = "ask"
	}
	root := cfg.WorkingDir
	if root != "" {
		if abs, err := filepath.Abs(root); err == nil {
			root = abs
		}
	}
	return &codingProvider{mode: mode, root: root, emit: emit, approve: cfg.ApproveFunc}
}

func (c *codingProvider) Tools() []rt.ToolDef {
	defs := []rt.ToolDef{
		toolDef("read_file", "Read a UTF-8 text file from the workspace. Use line_start/line_end (1-based, inclusive) to read only part of a large file.",
			`{"type":"object","properties":{"path":{"type":"string","description":"File path, relative to the workspace root or absolute."},"line_start":{"type":"integer","description":"First line (1-based, inclusive). Optional."},"line_end":{"type":"integer","description":"Last line (1-based, inclusive). Optional."}},"required":["path"]}`),
		toolDef("list_directory", "List files and folders at a path in the workspace.",
			`{"type":"object","properties":{"path":{"type":"string","description":"Directory path. Defaults to workspace root."}},"required":[]}`),
		toolDef("glob_search", "Find files matching a glob pattern (e.g. **/*.go).",
			`{"type":"object","properties":{"pattern":{"type":"string","description":"Glob pattern relative to workspace root."}},"required":["pattern"]}`),
		toolDef("grep_search", "Search file contents for a substring and return matching lines.",
			`{"type":"object","properties":{"query":{"type":"string"},"path":{"type":"string","description":"Optional sub-path to search. Defaults to workspace root."}},"required":["query"]}`),
		toolDef("write_file", "Create or overwrite a file with the given content. (requires approval)",
			`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		toolDef("edit_file", "Replace the first occurrence of old_text with new_text in a file. (requires approval)",
			`{"type":"object","properties":{"path":{"type":"string"},"old_text":{"type":"string"},"new_text":{"type":"string"}},"required":["path","old_text","new_text"]}`),
		toolDef("run_shell", "Run a shell command in the workspace and return its output. (requires approval)",
			`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`),
	}
	return defs
}

func toolDef(name, desc, schema string) rt.ToolDef {
	return rt.ToolDef{Type: "function", Function: rt.ToolFuncDef{
		Name: name, Description: desc, Parameters: json.RawMessage(schema),
	}}
}

func isRisky(name string) bool {
	switch name {
	case "write_file", "edit_file", "run_shell":
		return true
	}
	return false
}

func (c *codingProvider) Execute(name, argsJSON string) (string, error) {
	if isRisky(name) {
		switch c.mode {
		case "plan":
			return "", fmt.Errorf("blocked: in Plan mode the agent cannot modify files or run commands. Switch to Ask or Auto mode to apply changes")
		case "auto":
			// proceed without prompting
		default: // "ask"
			summary := riskSummary(name, argsJSON)
			if !c.requestApproval(name, summary, argsJSON) {
				return "", fmt.Errorf("denied by user")
			}
		}
	}

	switch name {
	case "read_file":
		return c.readFile(argsJSON)
	case "list_directory":
		return c.listDir(argsJSON)
	case "glob_search":
		return c.glob(argsJSON)
	case "grep_search":
		return c.grep(argsJSON)
	case "write_file":
		return c.writeFile(argsJSON)
	case "edit_file":
		return c.editFile(argsJSON)
	case "run_shell":
		return c.runShell(argsJSON)
	default:
		return "", fmt.Errorf("unknown coding tool: %s", name)
	}
}

func riskSummary(name, argsJSON string) string {
	var m map[string]interface{}
	json.Unmarshal([]byte(argsJSON), &m)
	switch name {
	case "run_shell":
		return fmt.Sprintf("Run command: %v", m["command"])
	case "write_file":
		return fmt.Sprintf("Overwrite file: %v", m["path"])
	case "edit_file":
		return fmt.Sprintf("Edit file: %v", m["path"])
	}
	return name
}

// requestApproval emits an approval_request event and blocks until resolved.
func (c *codingProvider) requestApproval(tool, summary, argsJSON string) bool {
	// CLI path: a synchronous approval callback (terminal y/n) instead of HTTP.
	if c.approve != nil {
		return c.approve(tool, summary, argsJSON)
	}
	c.mu.Lock()
	c.counter++
	id := fmt.Sprintf("appr_%d_%d", time.Now().UnixNano(), c.counter)
	c.mu.Unlock()

	ch := registerApproval(id)
	if c.emit != nil {
		c.emit("approval_request", map[string]interface{}{
			"id": id, "tool": tool, "summary": summary, "arguments": json.RawMessage(argsJSON),
		})
	}
	select {
	case ok := <-ch:
		return ok
	case <-time.After(5 * time.Minute):
		resolveApproval(id, false)
		return false
	}
}

// resolvePath maps a tool-supplied path into the workspace, enforcing containment
// when a root is configured.
func (c *codingProvider) resolvePath(p string) (string, error) {
	if p == "" {
		if c.root != "" {
			return c.root, nil
		}
		return ".", nil
	}
	var full string
	if filepath.IsAbs(p) {
		full = filepath.Clean(p)
	} else {
		base := c.root
		if base == "" {
			base = "."
		}
		full = filepath.Clean(filepath.Join(base, p))
	}
	if c.root != "" {
		rel, err := filepath.Rel(c.root, full)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("path %q is outside the workspace root", p)
		}
	}
	return full, nil
}

func (c *codingProvider) readFile(argsJSON string) (string, error) {
	var a struct {
		Path      string `json:"path"`
		LineStart *int   `json:"line_start"`
		LineEnd   *int   `json:"line_end"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if len(data) > 200*1024 {
		data = data[:200*1024]
	}
	// Honor an optional 1-based inclusive line range so the model can navigate
	// large files instead of re-reading the head.
	if a.LineStart != nil || a.LineEnd != nil {
		lines := strings.Split(string(data), "\n")
		start, end := 1, len(lines)
		if a.LineStart != nil {
			start = *a.LineStart
		}
		if a.LineEnd != nil {
			end = *a.LineEnd
		}
		if start < 1 {
			start = 1
		}
		if end > len(lines) {
			end = len(lines)
		}
		if start <= end {
			return strings.Join(lines[start-1:end], "\n"), nil
		}
	}
	return string(data), nil
}

func (c *codingProvider) listDir(argsJSON string) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", err
	}
	var items []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			name += "/"
		}
		items = append(items, name)
	}
	sort.Strings(items)
	b, _ := json.Marshal(map[string]interface{}{"path": full, "entries": items})
	return string(b), nil
}

func (c *codingProvider) glob(argsJSON string) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if a.Pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	base := c.root
	if base == "" {
		base = "."
	}
	var matches []string
	filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if isNoiseDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isBinaryName(path) {
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		rel = filepath.ToSlash(rel)
		if ok, _ := filepath.Match(a.Pattern, filepath.Base(path)); ok {
			matches = append(matches, rel)
		} else if matchGlobStar(a.Pattern, rel) {
			matches = append(matches, rel)
		}
		if len(matches) >= 500 {
			return filepath.SkipAll
		}
		return nil
	})
	b, _ := json.Marshal(map[string]interface{}{"matches": matches})
	return string(b), nil
}

// matchGlobStar handles simple ** patterns against a slash path.
func matchGlobStar(pattern, path string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}
	suffix := pattern[strings.LastIndex(pattern, "**")+2:]
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		return true
	}
	ok, _ := filepath.Match(suffix, filepath.Base(path))
	return ok
}

func (c *codingProvider) grep(argsJSON string) (string, error) {
	var a struct {
		Query string `json:"query"`
		Path  string `json:"path"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if a.Query == "" {
		return "", fmt.Errorf("query is required")
	}
	base, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	var hits []string
	filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if isNoiseDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if isBinaryName(path) {
			return nil
		}
		info, _ := d.Info()
		if info != nil && info.Size() > 2*1024*1024 {
			return nil
		}
		data, e := os.ReadFile(path)
		if e != nil {
			return nil
		}
		if strings.IndexByte(string(data), 0) >= 0 {
			return nil // binary file (null byte) — skip garbage matches
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, a.Query) {
				rel, _ := filepath.Rel(base, path)
				hits = append(hits, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), i+1, strings.TrimSpace(line)))
				if len(hits) >= 200 {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	b, _ := json.Marshal(map[string]interface{}{"matches": hits})
	return string(b), nil
}

func (c *codingProvider) writeFile(argsJSON string) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(a.Content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), full), nil
}

func (c *codingProvider) editFile(argsJSON string) (string, error) {
	var a struct {
		Path    string `json:"path"`
		OldText string `json:"old_text"`
		NewText string `json:"new_text"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	full, err := c.resolvePath(a.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	if !strings.Contains(string(data), a.OldText) {
		return "", fmt.Errorf("old_text not found in %s", a.Path)
	}
	updated := strings.Replace(string(data), a.OldText, a.NewText, 1)
	if err := os.WriteFile(full, []byte(updated), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s", full), nil
}

func (c *codingProvider) runShell(argsJSON string) (string, error) {
	var a struct {
		Command string `json:"command"`
	}
	json.Unmarshal([]byte(argsJSON), &a)
	if strings.TrimSpace(a.Command) == "" {
		return "", fmt.Errorf("command is required")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", a.Command)
	} else {
		cmd = exec.Command("sh", "-c", a.Command)
	}
	if c.root != "" {
		cmd.Dir = c.root
	}
	out, err := cmd.CombinedOutput()
	res := string(out)
	if len(res) > 100*1024 {
		res = res[:100*1024] + "\n...[truncated]"
	}
	if err != nil {
		return res, fmt.Errorf("command exited with error: %v", err)
	}
	return res, nil
}
