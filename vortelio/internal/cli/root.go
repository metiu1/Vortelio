package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/vortelio/vortelio/internal/cli/commands"
	"github.com/vortelio/vortelio/internal/version"
	"github.com/vortelio/vortelio/pkg/progress"
)

func Execute() error {
	progress.EnableANSI()
	root := newRootCommand()
	root.addCommands(
		commands.NewPullCommand(),
		commands.NewRunCommand(),
		commands.NewListCommand(),
		commands.NewRemoveCommand(),
		commands.NewQuantizeCommand(),
		commands.NewInfoCommand(),
		commands.NewServeCommand(),
		commands.NewGUICommand(),
		commands.NewSetupCommand(),
		commands.NewCleanupCommand(),
		commands.NewImportOllamaCommand(),
	)
	return root.run(os.Args[1:])
}

type rootCommand struct {
	subcommands []commands.Command
}

func newRootCommand() *rootCommand { return &rootCommand{} }
func (r *rootCommand) addCommands(cmds ...commands.Command) {
	r.subcommands = append(r.subcommands, cmds...)
}

func (r *rootCommand) run(args []string) error {
	if len(args) == 0 {
		if err := runInteractiveMenu(); err != nil {
			r.printHelp()
		}
		return nil
	}
	name := args[0]
	if name == "--help" || name == "-h" || name == "help" { r.printHelp(); return nil }
	if name == "--version" || name == "-v" || name == "version" {
		fmt.Printf("vortelio version %s\n", version.Version)
		return nil
	}
	aliasMap := map[string]string{"rm": "remove", "ls": "list", "ps": "list", "start": "run"}
	if aliasTarget, ok := aliasMap[name]; ok {
		for _, cmd := range r.subcommands {
			if cmd.Name() == aliasTarget { return cmd.Run(args[1:]) }
		}
	}
	for _, cmd := range r.subcommands {
		if cmd.Name() == name { return cmd.Run(args[1:]) }
	}
	if strings.Contains(name, "/") {
		for _, cmd := range r.subcommands {
			if cmd.Name() == "run" { return cmd.Run(args) }
		}
	}
	fmt.Fprintf(os.Stderr, "❌  Unknown command: %q\n\nUse 'vortelio help' for the list of commands.\n", name)
	return nil
}

func (r *rootCommand) printHelp() {
	fmt.Printf("\nVortelio %s — run AI models locally\n\n", version.Version)

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("MAIN COMMANDS")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  vortelio pull     <model>          Download a model from HuggingFace")
	fmt.Println("  vortelio run      <model> [prompt] Run a model")
	fmt.Println("  vortelio list                      List downloaded models")
	fmt.Println("  vortelio rm       <model>          Remove a model")
	fmt.Println("  vortelio rm       --all            Remove all models")
	fmt.Println("  vortelio info     <model>          Model details")
	fmt.Println("  vortelio gui                       Open the Web UI in the browser")
	fmt.Println("  vortelio serve    [--port N]       Start Web UI (default port 11500)")
	fmt.Println("  vortelio quantize <model>          Quantize a model")
	fmt.Println("  vortelio setup                     Install dependencies (llama.cpp, Python)")
	fmt.Println("  vortelio cleanup                   Analyze disk space")
	fmt.Println("  vortelio cleanup  --delete         Delete unnecessary files")
	fmt.Println("  vortelio help                      Show this message")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("MODEL FORMAT (type prefix required)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Types:   llm | image | audio | video | 3d")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("EXAMPLES — ONE PER TYPE")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Println("  💬  LLM (text chat):")
	fmt.Println("      vortelio pull llm/mistral:7b")
	fmt.Println("      vortelio run  llm/mistral:7b \"Explain how a jet engine works\"")
	fmt.Println()
	fmt.Println("  🎨  IMAGE (generate images):")
	fmt.Println("      vortelio pull image/dreamshaper:latest")
	fmt.Println("      vortelio run  image/dreamshaper:latest \"a sunset on Mars, artistic style\"")
	fmt.Println()
	fmt.Println("  🔊  AUDIO — Transcription (Whisper):")
	fmt.Println("      vortelio pull audio/whisper:large-v3")
	fmt.Println("      vortelio run  audio/whisper:large-v3 --input recording.mp3")
	fmt.Println()
	fmt.Println("  🔊  AUDIO — Text-to-speech (Kokoro TTS):")
	fmt.Println("      vortelio pull audio/kokoro:latest")
	fmt.Println("      vortelio run  audio/kokoro:latest \"Hello, I am Vortelio!\" --output voice.wav")
	fmt.Println()
	fmt.Println("  🎬  VIDEO (generate video):")
	fmt.Println("      vortelio pull video/wan:1.3b")
	fmt.Println("      vortelio run  video/wan:1.3b \"a cat flying among the clouds\"")
	fmt.Println()
	fmt.Println("  🧊  3D (generate 3D mesh):")
	fmt.Println("      vortelio pull 3d/triposr:latest")
	fmt.Println("      vortelio run  3d/triposr:latest --input photo.jpg")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("AI AGENTS (OpenClaw, Open Code)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Agents are installed and started from the Vortelio Web UI.")
	fmt.Println("  Open the GUI with:  vortelio gui")
	fmt.Println("  Then go to:         AI Agents → Install → Start → Open web interface")
	fmt.Println()
	fmt.Println("  Or start them manually (after npm install -g <package>):")
	fmt.Println()
	fmt.Println("  🦞  OpenClaw (multi-channel AI gateway, port 18789):")
	fmt.Println("      openclaw gateway start")
	fmt.Println("      Then open: http://localhost:18789")
	fmt.Println()
	fmt.Println("  💻  Open Code (coding agent, port 4096):")
	fmt.Println("      opencode web --port 4096")
	fmt.Println("      Then open: http://localhost:4096")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("CLOUD MODELS (OpenAI, Anthropic, Gemini, Groq, Mistral...)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  Cloud models are used from the Vortelio Web UI:")
	fmt.Println("  vortelio gui")
	fmt.Println("  Then go to: ☁️ Cloud Models → choose provider → enter API key → Chat")
	fmt.Println()
	fmt.Println("  Supported providers:")
	fmt.Println("    🦙  Ollama Cloud  → https://ollama.com/settings/keys")
	fmt.Println("    🟢  OpenAI        → https://platform.openai.com/api-keys")
	fmt.Println("    🧠  Anthropic      → https://console.anthropic.com/keys")
	fmt.Println("    ♊  Google Gemini  → https://aistudio.google.com/app/apikey")
	fmt.Println("    ⚡  Groq           → https://console.groq.com/keys")
	fmt.Println("    🌬️  Mistral AI     → https://console.mistral.ai/api-keys")
	fmt.Println("    🔀  OpenRouter     → https://openrouter.ai/keys")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("DOWNLOAD FROM HUGGINGFACE")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  ⚠️  The type prefix is REQUIRED for URL downloads:")
	fmt.Println("      ✓  vortelio pull llm/https://huggingface.co/owner/repo")
	fmt.Println("      ✓  vortelio pull llm/hf.co/unsloth/Qwen2.5-0.5B-Instruct-GGUF:Q4_K_M")
	fmt.Println("      ✗  vortelio pull https://huggingface.co/owner/repo  ← missing 'llm/'")
}
