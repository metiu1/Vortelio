package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
	"github.com/vortelio/vortelio/internal/server"
)

// CodeCommand is a terminal coding agent (like Claude Code) built on the exact
// same harness as the Developer GUI: the same agentic tool loop, coding tools
// (read/write/edit/run), web search and self-authored skills.
type CodeCommand struct{}

func NewCodeCommand() *CodeCommand { return &CodeCommand{} }

func (c *CodeCommand) Name() string { return "code" }

func (c *CodeCommand) Run(args []string) error {
	modelRef := ""
	workdir, _ := os.Getwd()
	autonomous := false
	forceCPU := false
	var firstPrompt []string

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model", "-m":
			if i+1 < len(args) {
				modelRef = args[i+1]
				i++
			}
		case "--dir", "-d":
			if i+1 < len(args) {
				workdir = args[i+1]
				i++
			}
		case "--auto", "--autonomous":
			autonomous = true
		case "--cpu":
			forceCPU = true
		case "--help", "-h":
			printCodeHelp()
			return nil
		default:
			if !strings.HasPrefix(args[i], "--") {
				firstPrompt = append(firstPrompt, args[i])
			}
		}
	}

	store := hub.NewModelStore()
	var model *hub.Model
	if modelRef != "" {
		ref, err := hub.ParseModelRef(modelRef)
		if err != nil {
			return fmt.Errorf("modello non valido: %w", err)
		}
		model, err = store.Resolve(ref)
		if err != nil {
			return fmt.Errorf("modello non trovato: %w (scaricalo con: vortelio pull %s)", err, modelRef)
		}
	} else {
		model = pickDefaultLLM(store)
		if model == nil {
			return fmt.Errorf("nessun modello LLM installato.\n  Scaricane uno tool-capable, es:  vortelio pull llm/qwen2.5:7b")
		}
	}

	hw := runtime.DetectHardware()
	if forceCPU {
		hw.Backend = runtime.BackendCPU
	}

	fmt.Printf("\n\033[1m🧑‍💻 Vortelio Code\033[0m — agente coding nel terminale (stesso motore del Developer)\n")
	fmt.Printf("   modello: \033[36m%s/%s:%s\033[0m   cartella: \033[36m%s\033[0m%s\n", model.Type, model.Name, model.Tag, workdir,
		map[bool]string{true: "   \033[33m[AUTONOMO]\033[0m", false: ""}[autonomous])
	fmt.Printf("   comandi: \033[2m/exit per uscire · /auto per attivare la modalità autonoma · /cd <dir>\033[0m\n")
	fmt.Printf("   \033[2mscrivi un obiettivo o una richiesta e premi invio…\033[0m\n")

	runner, err := runtime.GlobalModelManager.GetOrLoad(model, hw, 30*time.Minute)
	if err != nil {
		return fmt.Errorf("caricamento modello fallito: %w", err)
	}

	emit := func(ev string, data interface{}) {
		b, _ := json.Marshal(data)
		var m map[string]interface{}
		_ = json.Unmarshal(b, &m)
		switch ev {
		case "tool_call":
			fmt.Printf("\n  \033[36m⚙ %v\033[0m \033[2m%s\033[0m\n", m["name"], truncStr(fmt.Sprint(m["arguments"]), 140))
		case "tool_result":
			if e, ok := m["error"].(string); ok && e != "" {
				fmt.Printf("  \033[31m✕ %s\033[0m\n", truncStr(e, 200))
			} else {
				fmt.Printf("  \033[32m✓\033[0m \033[2m%s\033[0m\n", truncStr(fmt.Sprint(m["result"]), 200))
			}
		case "approval_request":
			fmt.Printf("  \033[33m⚠ richiesta azione: %v (eseguita in modalità auto)\033[0m\n", m["tool"])
		case "media_generated":
			fmt.Printf("  \033[35m🎨 generato: %v\033[0m\n", m["path"])
		}
	}

	harness := server.BuildCodingHarness(workdir, "auto", autonomous, emit)
	system := server.CodingSystemPrompt(autonomous)

	var messages []map[string]interface{}
	reader := bufio.NewReader(os.Stdin)

	runTurn := func(line string) {
		messages = append(messages, map[string]interface{}{"role": "user", "content": line})
		sopts := runtime.StreamOpts{
			System:       system,
			Messages:     messages,
			ToolsEnabled: true,
			ToolProvider: harness,
		}
		if autonomous {
			sopts.MaxToolRounds = 40
		}
		fmt.Print("\n")
		var resp strings.Builder
		err := runner.StreamWithOpts(sopts, func(tok string) {
			fmt.Print(tok)
			resp.WriteString(tok)
		}, emit)
		fmt.Print("\n")
		if err != nil {
			fmt.Printf("\033[31m✕ errore: %v\033[0m\n", err)
			return
		}
		messages = append(messages, map[string]interface{}{"role": "assistant", "content": resp.String()})
	}

	if len(firstPrompt) > 0 {
		runTurn(strings.Join(firstPrompt, " "))
	}

	for {
		fmt.Print("\n\033[1m›\033[0m ")
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case line == "/exit" || line == "/quit":
			return nil
		case line == "/auto":
			autonomous = !autonomous
			harness = server.BuildCodingHarness(workdir, "auto", autonomous, emit)
			system = server.CodingSystemPrompt(autonomous)
			fmt.Printf("  \033[33mmodalità autonoma: %v\033[0m\n", autonomous)
			continue
		case strings.HasPrefix(line, "/cd "):
			workdir = strings.TrimSpace(line[4:])
			harness = server.BuildCodingHarness(workdir, "auto", autonomous, emit)
			fmt.Printf("  \033[36mcartella: %s\033[0m\n", workdir)
			continue
		}
		runTurn(line)
	}
	return nil
}

// pickDefaultLLM returns the configured default LLM, else the first installed,
// preferring a tool-capable one.
func pickDefaultLLM(store *hub.ModelStore) *hub.Model {
	models, err := store.List()
	if err != nil {
		return nil
	}
	var firstLLM *hub.Model
	for _, m := range models {
		if m.Type != "llm" {
			continue
		}
		if firstLLM == nil {
			firstLLM = m
		}
		if runtime.ModelSupportsTools(m.Name + ":" + m.Tag) {
			return m
		}
	}
	return firstLLM
}

func truncStr(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func printCodeHelp() {
	fmt.Println("vortelio code — agente coding nel terminale (stesso motore del Developer GUI)")
	fmt.Println("")
	fmt.Println("Uso:  vortelio code [obiettivo] [flag]")
	fmt.Println("")
	fmt.Println("Flag:")
	fmt.Println("  -m, --model <ref>   Modello da usare (default: primo LLM tool-capable installato)")
	fmt.Println("  -d, --dir <path>    Cartella di lavoro (default: cartella corrente)")
	fmt.Println("      --auto          Modalità autonoma: lavora da solo verso l'obiettivo")
	fmt.Println("      --cpu           Forza CPU")
	fmt.Println("")
	fmt.Println("Comandi in chat:  /exit  /auto  /cd <dir>")
	fmt.Println("")
	fmt.Println("Esempi:")
	fmt.Println("  vortelio code")
	fmt.Println("  vortelio code \"crea un'API Flask con una rotta /ping\" --auto")
	fmt.Println("  vortelio code -m llm/qwen2.5:7b -d ./mioprogetto")
}
