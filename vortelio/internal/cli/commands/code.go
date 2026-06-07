package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
	"github.com/vortelio/vortelio/internal/server"
)

// ANSI helpers
const (
	cReset = "\033[0m"
	cDim   = "\033[2m"
	cBold  = "\033[1m"
	cCyan  = "\033[36m"
	cGreen = "\033[32m"
	cYell  = "\033[33m"
	cRed   = "\033[31m"
	cMag   = "\033[35m"
	cBlue  = "\033[34m"
)

// CodeCommand is the Vortelio terminal coding agent, on the same harness
// as the Developer GUI: agentic tool loop, coding tools, web, media, skills, MCP.
type CodeCommand struct{}

func NewCodeCommand() *CodeCommand { return &CodeCommand{} }
func (c *CodeCommand) Name() string { return "code" }

type codeSession struct {
	model      *hub.Model
	runner     *runtime.LLMRunner
	hw         *runtime.Hardware
	workdir    string
	autonomous bool
	mcpOn      bool
	skills     []string
	messages   []map[string]interface{}
}

func (c *CodeCommand) Run(args []string) error {
	s := &codeSession{}
	s.workdir, _ = os.Getwd()
	var modelRef string
	var firstPrompt []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model", "-m":
			if i+1 < len(args) { modelRef = args[i+1]; i++ }
		case "--dir", "-d":
			if i+1 < len(args) { s.workdir = args[i+1]; i++ }
		case "--auto", "--autonomous":
			s.autonomous = true
		case "--cpu":
			// handled after hardware detect
			args[i] = "--cpu"
		case "--help", "-h":
			printCodeHelp(); return nil
		default:
			if !strings.HasPrefix(args[i], "--") { firstPrompt = append(firstPrompt, args[i]) }
		}
	}

	store := hub.NewModelStore()
	if modelRef != "" {
		ref, err := hub.ParseModelRef(modelRef)
		if err != nil { return fmt.Errorf("modello non valido: %w", err) }
		s.model, err = store.Resolve(ref)
		if err != nil { return fmt.Errorf("modello non trovato: %w (scaricalo: vortelio pull %s)", err, modelRef) }
	} else {
		s.model = pickDefaultLLM(store)
		if s.model == nil {
			return fmt.Errorf("nessun LLM installato. Scarica un modello tool-capable:\n  vortelio pull llm/qwen2.5:7b")
		}
	}
	s.hw = runtime.DetectHardware()
	for _, a := range args { if a == "--cpu" { s.hw.Backend = runtime.BackendCPU } }

	printBanner(s)
	if err := s.loadModel(); err != nil { return err }

	reader := bufio.NewReader(os.Stdin)
	if len(firstPrompt) > 0 { s.runTurn(strings.Join(firstPrompt, " ")) }

	for {
		fmt.Printf("\n%s%s›%s ", cBold, cCyan, cReset)
		line, err := reader.ReadString('\n')
		if err != nil { fmt.Println(); break }
		line = strings.TrimSpace(line)
		if line == "" { continue }
		if line == "@" { s.listWorkdirFiles(); continue }
		if line == "/" { printSlashHelp(); continue }
		if strings.HasPrefix(line, "/") {
			if s.handleCommand(line, reader) { return nil }
			continue
		}
		s.runTurn(line)
	}
	return nil
}

func (s *codeSession) loadModel() error {
	r, err := runtime.GlobalModelManager.GetOrLoad(s.model, s.hw, 30*time.Minute)
	if err != nil { return fmt.Errorf("caricamento modello fallito: %w", err) }
	s.runner = r
	return nil
}

func (s *codeSession) emit(ev string, data interface{}) {
	b, _ := json.Marshal(data)
	var m map[string]interface{}
	_ = json.Unmarshal(b, &m)
	switch ev {
	case "tool_call":
		fmt.Printf("\n  %s⚙ %v%s %s%s%s\n", cCyan, m["name"], cReset, cDim, truncStr(fmt.Sprint(m["arguments"]), 140), cReset)
	case "tool_result":
		if e, ok := m["error"].(string); ok && e != "" {
			fmt.Printf("  %s✕ %s%s\n", cRed, truncStr(e, 200), cReset)
		} else {
			fmt.Printf("  %s✓%s %s%s%s\n", cGreen, cReset, cDim, truncStr(fmt.Sprint(m["result"]), 200), cReset)
		}
	case "media_generated":
		fmt.Printf("  %s🎨 generato: %v%s\n", cMag, m["path"], cReset)
	}
}

func (s *codeSession) runTurn(line string) {
	line = s.expandFileRefs(line)
	s.messages = append(s.messages, map[string]interface{}{"role": "user", "content": line})
	prov, sys := server.BuildCLIHarness(s.workdir, "auto", s.autonomous, s.mcpOn, s.skills, s.emit)
	sopts := runtime.StreamOpts{System: sys, Messages: s.messages, ToolsEnabled: true, ToolProvider: prov}
	if s.autonomous { sopts.MaxToolRounds = 40 }

	t0 := time.Now()
	fmt.Print("\n")
	var resp strings.Builder
	err := s.runner.StreamWithOpts(sopts, func(tok string) { fmt.Print(tok); resp.WriteString(tok) }, s.emit)
	fmt.Print("\n")
	if err != nil { fmt.Printf("%s✕ errore: %v%s\n", cRed, err, cReset); return }
	secs := time.Since(t0).Seconds()
	tokens := len(resp.String())/4 + 1
	fmt.Printf("%s⏱ %.1fs · ~%d token · %s%s\n", cDim, secs, tokens, s.model.Name+":"+s.model.Tag, cReset)
	s.messages = append(s.messages, map[string]interface{}{"role": "assistant", "content": resp.String()})
}

// expandFileRefs replaces @path tokens with the file's content so the model can
// read referenced files directly.
var fileRefRE = regexp.MustCompile(`@([^\s"']+)`)

func (s *codeSession) expandFileRefs(line string) string {
	var extras []string
	out := fileRefRE.ReplaceAllStringFunc(line, func(tok string) string {
		p := tok[1:]
		full := p
		if !filepath.IsAbs(p) { full = filepath.Join(s.workdir, p) }
		data, err := os.ReadFile(full)
		if err != nil { return tok } // leave as-is if unreadable
		if len(data) > 40000 { data = data[:40000] }
		extras = append(extras, fmt.Sprintf("\n\n--- File \"%s\" ---\n%s", p, string(data)))
		fmt.Printf("  %s📎 incluso %s (%d byte)%s\n", cDim, p, len(data), cReset)
		return p
	})
	return out + strings.Join(extras, "")
}

// handleCommand returns true when the session should exit.
func (s *codeSession) handleCommand(line string, reader *bufio.Reader) bool {
	parts := strings.Fields(line)
	cmd := parts[0]
	switch cmd {
	case "/exit", "/quit", "/q":
		return true
	case "/help", "/?":
		printSlashHelp()
	case "/clear":
		s.messages = nil
		fmt.Printf("  %scontesto azzerato%s\n", cDim, cReset)
	case "/auto":
		s.autonomous = !s.autonomous
		fmt.Printf("  %smodalità autonoma: %v%s\n", cYell, s.autonomous, cReset)
	case "/mcp":
		s.mcpOn = !s.mcpOn
		fmt.Printf("  %sMCP: %v%s\n", cYell, s.mcpOn, cReset)
	case "/cd":
		if len(parts) > 1 { s.workdir = strings.TrimSpace(line[len("/cd "):]); fmt.Printf("  %scartella: %s%s\n", cCyan, s.workdir, cReset) }
	case "/model", "/m":
		s.chooseModel(reader)
	case "/skills", "/skill":
		s.chooseSkills(reader)
	default:
		fmt.Printf("  %scomando sconosciuto: %s — /help per la lista%s\n", cDim, cmd, cReset)
	}
	return false
}

func (s *codeSession) chooseModel(reader *bufio.Reader) {
	models, _ := hub.NewModelStore().List()
	var llms []*hub.Model
	for _, m := range models { if m.Type == "llm" { llms = append(llms, m) } }
	if len(llms) == 0 { fmt.Printf("  %snessun LLM installato%s\n", cDim, cReset); return }
	fmt.Printf("\n  %sModelli disponibili:%s\n", cBold, cReset)
	for i, m := range llms {
		mark := " "
		if m.Name == s.model.Name && m.Tag == s.model.Tag { mark = cGreen + "●" + cReset }
		tools := ""
		if runtime.ModelSupportsTools(m.Name + ":" + m.Tag) { tools = cDim + " 🛠" + cReset }
		fmt.Printf("   %s %d) %s:%s%s\n", mark, i+1, m.Name, m.Tag, tools)
	}
	fmt.Printf("  %snumero del modello (invio per annullare):%s ", cDim, cReset)
	in, _ := reader.ReadString('\n')
	in = strings.TrimSpace(in)
	var idx int
	if _, err := fmt.Sscanf(in, "%d", &idx); err != nil || idx < 1 || idx > len(llms) { return }
	s.model = llms[idx-1]
	fmt.Printf("  %s⏳ carico %s:%s…%s\n", cDim, s.model.Name, s.model.Tag, cReset)
	if err := s.loadModel(); err != nil { fmt.Printf("  %s%v%s\n", cRed, err, cReset) } else {
		fmt.Printf("  %s✓ modello: %s:%s%s\n", cGreen, s.model.Name, s.model.Tag, cReset)
	}
}

func (s *codeSession) chooseSkills(reader *bufio.Reader) {
	all := server.ListSkillInfos()
	if len(all) == 0 { fmt.Printf("  %snessuna skill%s\n", cDim, cReset); return }
	on := map[string]bool{}
	for _, id := range s.skills { on[id] = true }
	fmt.Printf("\n  %sSkill (numero per attivare/disattivare, invio per chiudere):%s\n", cBold, cReset)
	for i, sk := range all {
		box := "[ ]"
		if on[sk.ID] { box = cGreen + "[x]" + cReset }
		fmt.Printf("   %s %d) %s\n", box, i+1, sk.Name)
	}
	fmt.Printf("  %s>%s ", cDim, cReset)
	in, _ := reader.ReadString('\n')
	in = strings.TrimSpace(in)
	var idx int
	if _, err := fmt.Sscanf(in, "%d", &idx); err != nil || idx < 1 || idx > len(all) { return }
	id := all[idx-1].ID
	if on[id] {
		var ns []string
		for _, x := range s.skills { if x != id { ns = append(ns, x) } }
		s.skills = ns
		fmt.Printf("  %sskill disattivata: %s%s\n", cDim, all[idx-1].Name, cReset)
	} else {
		s.skills = append(s.skills, id)
		fmt.Printf("  %s✓ skill attivata: %s%s\n", cGreen, all[idx-1].Name, cReset)
	}
}

func (s *codeSession) listWorkdirFiles() {
	entries, err := os.ReadDir(s.workdir)
	if err != nil { fmt.Printf("  %scartella non leggibile%s\n", cDim, cReset); return }
	fmt.Printf("\n  %sFile in %s (usa @nome per allegarli):%s\n", cBold, s.workdir, cReset)
	n := 0
	for _, e := range entries {
		if n >= 40 { fmt.Printf("   %s… e altri%s\n", cDim, cReset); break }
		if e.IsDir() { fmt.Printf("   %s📁 %s/%s\n", cDim, e.Name(), cReset) } else { fmt.Printf("   📄 %s\n", e.Name()) }
		n++
	}
}

func printBanner(s *codeSession) {
	fmt.Printf("\n%s%s ┌─────────────────────────────────────────────┐%s\n", cBold, cCyan, cReset)
	fmt.Printf("%s%s │  🧑‍💻 Vortelio Code                            │%s\n", cBold, cCyan, cReset)
	fmt.Printf("%s%s └─────────────────────────────────────────────┘%s\n", cBold, cCyan, cReset)
	fmt.Printf("   %smodello%s %s%s:%s%s   %scartella%s %s%s\n", cDim, cReset, cCyan, s.model.Name, s.model.Tag, cReset, cDim, cReset, s.workdir,
		map[bool]string{true: "   " + cYell + "[AUTONOMO]" + cReset, false: ""}[s.autonomous])
	fmt.Printf("   %sscrivi un obiettivo · %s/help%s%s per i comandi · %s@file%s%s per allegare un file%s\n", cDim, cReset, cDim, cReset, cReset, cDim, cReset, cReset)
}

func printSlashHelp() {
	fmt.Printf("\n  %sComandi:%s\n", cBold, cReset)
	fmt.Println("   /model      cambia modello LLM")
	fmt.Println("   /skills     attiva/disattiva skill")
	fmt.Println("   /mcp        attiva/disattiva i tool MCP")
	fmt.Println("   /auto       modalità autonoma (loop verso l'obiettivo)")
	fmt.Println("   /cd <dir>   cambia cartella di lavoro")
	fmt.Println("   /clear      azzera il contesto")
	fmt.Println("   /help       questo messaggio")
	fmt.Println("   /exit       esci")
	fmt.Printf("  %s@percorso/file  →  include il contenuto del file nel messaggio%s\n", cDim, cReset)
}

func pickDefaultLLM(store *hub.ModelStore) *hub.Model {
	models, err := store.List()
	if err != nil { return nil }
	var firstLLM *hub.Model
	for _, m := range models {
		if m.Type != "llm" { continue }
		if firstLLM == nil { firstLLM = m }
		if runtime.ModelSupportsTools(m.Name + ":" + m.Tag) { return m }
	}
	return firstLLM
}

func truncStr(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max { return s[:max] + "…" }
	return s
}

func printCodeHelp() {
	fmt.Println("vortelio code — agente coding nel terminale (stesso motore del Developer GUI)")
	fmt.Println("")
	fmt.Println("Uso:  vortelio code [obiettivo] [flag]")
	fmt.Println("")
	fmt.Println("Flag:")
	fmt.Println("  -m, --model <ref>   Modello (default: primo LLM tool-capable installato)")
	fmt.Println("  -d, --dir <path>    Cartella di lavoro (default: cartella corrente)")
	fmt.Println("      --auto          Modalità autonoma: lavora da solo verso l'obiettivo")
	fmt.Println("      --cpu           Forza CPU")
	fmt.Println("")
	fmt.Println("In chat:  /model  /skills  /mcp  /auto  /cd  /clear  /help  /exit  ·  @file")
	fmt.Println("")
	fmt.Println("Esempi:")
	fmt.Println("  vortelio code")
	fmt.Println("  vortelio code \"crea un'API Flask con /ping\" --auto")
	fmt.Println("  vortelio code -m llm/qwen2.5:7b -d ./progetto")
}
