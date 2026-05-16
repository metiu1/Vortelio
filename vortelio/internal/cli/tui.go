package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/vortelio/vortelio/internal/config"
	"github.com/vortelio/vortelio/internal/hub"
	"golang.org/x/term"
)

// ─── model types ─────────────────────────────────────────────────────────────

var modelTypes = []struct {
	key   string
	label string
}{
	{"llm", "Language (LLM)"},
	{"image", "Image"},
	{"audio", "Audio"},
	{"video", "Video"},
	{"3d", "3D"},
}

func typeLabels() []string {
	out := make([]string, len(modelTypes))
	for i, t := range modelTypes {
		out[i] = t.label
	}
	return out
}

// ─── main menu ───────────────────────────────────────────────────────────────

func runInteractiveMenu() error {
	for {
		sel := selectMenu("Vortelio", []string{
			"Chat with a model",
			"Download a model",
			"Cloud Models",
			"AI Agents",
			"Import from Ollama",
			"Advanced Tools",
			"Open Web UI",
			"Show commands",
		})
		switch sel {
		case -1:
			os.Exit(0)
		case 0:
			if err := handleChatta(); err != nil {
				return err
			}
		case 1:
			if err := handleScarica(); err != nil {
				return err
			}
		case 2:
			if err := handleModelloCloud(); err != nil {
				return err
			}
		case 3:
			if err := handleAgentiAI(); err != nil {
				return err
			}
		case 4:
			if err := handleImportOllama(); err != nil {
				return err
			}
		case 5:
			if err := handleAdvancedTools(); err != nil {
				return err
			}
		case 6:
			return reExec("gui")
		case 7:
			return reExec("help")
		}
	}
}

// ─── chat ─────────────────────────────────────────────────────────────────────

func handleChatta() error {
	for {
		tSel := selectMenu("Model type", typeLabels())
		if tSel < 0 {
			return nil // back to main menu
		}
		chosen := modelTypes[tSel]

		// installed models of that type
		store := hub.NewModelStore()
		all, _ := store.List()
		var models []*hub.Model
		for _, m := range all {
			if m.Type == chosen.key {
				models = append(models, m)
			}
		}

		if len(models) == 0 {
			waitKey(fmt.Sprintf(
				"  No %s model installed.\n  Use «Download a model» to get one.",
				chosen.label,
			))
			continue // back to type selection
		}

		labels := make([]string, len(models))
		for i, m := range models {
			name := m.DisplayName
			if name == "" {
				name = m.Name + ":" + m.Tag
			}
			labels[i] = fmt.Sprintf("%-36s %s", name, m.SizeHuman())
		}

		mSel := selectMenu("Select model — "+chosen.label, labels)
		if mSel < 0 {
			continue // back to type selection
		}

		m := models[mSel]
		fmt.Print("\033[H\033[2J")
		return reExec("run", fmt.Sprintf("%s/%s:%s", m.Type, m.Name, m.Tag))
	}
}

// ─── download ────────────────────────────────────────────────────────────────

func handleScarica() error {
	tSel := selectMenu("Model type", typeLabels())
	if tSel < 0 {
		return nil
	}
	chosen := modelTypes[tSel]

	fmt.Print("\033[H\033[2J")
	fmt.Printf("\n  %s model (e.g. %s/hf.co/owner/repo:file)\n  > ", chosen.label, chosen.key)
	var ref string
	fmt.Scanln(&ref)
	if ref != "" {
		// se l'utente non ha già il prefisso tipo, aggiungilo
		if len(ref) < 3 || ref[:len(chosen.key)+1] != chosen.key+"/" {
			ref = chosen.key + "/" + ref
		}
		return reExec("pull", ref)
	}
	return nil
}

// ─── import from Ollama ──────────────────────────────────────

func handleImportOllama() error {
	sel := selectMenu("Import from Ollama", []string{
		"Import all Ollama models",
		"Dry run (preview only)",
		"← Back",
	})
	if sel < 0 || sel == 2 {
		return nil
	}
	dryRun := sel == 1
	fmt.Print("\033[H\033[2J")
	fmt.Println()
	if dryRun {
		fmt.Println("  Scanning Ollama models (dry run)…")
	} else {
		fmt.Println("  Importing Ollama models…")
	}
	fmt.Println()

	port := fmt.Sprintf("%d", config.Get().Port)
	url := fmt.Sprintf("http://localhost:%s/api/import/ollama", port)
	body, _ := json.Marshal(map[string]interface{}{"dry_run": dryRun})
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		waitKey(fmt.Sprintf("  ❌  Could not reach Vortelio server:\n  %s\n\n  Make sure the server is running (vortelio serve).", err.Error()))
		return nil
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		waitKey("  ❌  Invalid response from server.")
		return nil
	}
	if errMsg, ok := result["error"].(string); ok {
		waitKey(fmt.Sprintf("  ❌  %s", errMsg))
		return nil
	}

	imported, _ := result["imported"].([]interface{})
	skipped, _ := result["skipped"].([]interface{})
	ollamaPath, _ := result["ollama_path"].(string)
	dryLabel := ""
	if dryRun {
		dryLabel = " (dry run)"
	}
	fmt.Printf("  Ollama path: %s%s\n\n", ollamaPath, dryLabel)
	action := "Imported"
	if dryRun {
		action = "Would import"
	}
	fmt.Printf("  ✅  %s: %d model(s)\n", action, len(imported))
	for _, m := range imported {
		if mm, ok := m.(map[string]interface{}); ok {
			fmt.Printf("      + %v\n", mm["model"])
		}
	}
	if len(skipped) > 0 {
		fmt.Printf("\n  Skipped: %d\n", len(skipped))
		for _, s := range skipped {
			if sm, ok := s.(map[string]interface{}); ok {
				fmt.Printf("      · %v — %v\n", sm["model"], sm["reason"])
			}
		}
	}
	waitKey("")
	return nil
}

// ─── advanced tools ───────────────────────────────────────────

func handleAdvancedTools() error {
	for {
		sel := selectMenu("Advanced Tools", []string{
			"📚 RAG — query a collection",
			"🔍 GGUF Inspect",
			"📋 Audit Log (last 20)",
			"← Back",
		})
		switch sel {
		case -1, 3:
			return nil
		case 0:
			handleRAGQuery()
		case 1:
			handleGGUFInspect()
		case 2:
			handleAuditLog()
		}
	}
}

func apiCall(method, endpoint string, payload interface{}) (map[string]interface{}, error) {
	port := fmt.Sprintf("%d", config.Get().Port)
	url := fmt.Sprintf("http://localhost:%s%s", port, endpoint)
	var resp *http.Response
	var err error
	if payload != nil {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(method, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err = http.DefaultClient.Do(req)
	} else {
		resp, err = http.Get(url)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}

func handleRAGQuery() {
	fmt.Print("\033[H\033[2J")
	fmt.Println("\n  📚 RAG Query")
	fmt.Println()
	fmt.Print("  Embed model (e.g. llm/nomic-embed-text:latest): ")
	model := strings.TrimSpace(readLineSimple())
	if model == "" {
		return
	}
	fmt.Print("  Collection name [default]: ")
	coll := strings.TrimSpace(readLineSimple())
	if coll == "" {
		coll = "default"
	}
	fmt.Print("  Query: ")
	query := strings.TrimSpace(readLineSimple())
	if query == "" {
		return
	}

	fmt.Println("\n  Searching…")
	result, err := apiCall("POST", "/api/rag/query", map[string]interface{}{
		"model": model, "collection": coll, "query": query, "top_k": 5,
	})
	if err != nil {
		waitKey(fmt.Sprintf("  ❌  %s", err.Error()))
		return
	}
	if errMsg, ok := result["error"].(string); ok {
		waitKey(fmt.Sprintf("  ❌  %s", errMsg))
		return
	}

	results, _ := result["results"].([]interface{})
	if len(results) == 0 {
		waitKey("  No results found.")
		return
	}
	fmt.Print("\033[H\033[2J")
	fmt.Printf("\n  📚 RAG Results for: %q\n", query)
	fmt.Println()
	for i, r := range results {
		if rm, ok := r.(map[string]interface{}); ok {
			score, _ := rm["score"].(float64)
			text, _ := rm["text"].(string)
			if len(text) > 200 {
				text = text[:197] + "…"
			}
			fmt.Printf("  #%d  score: %.4f\n", i+1, score)
			fmt.Printf("      %s\n\n", strings.ReplaceAll(text, "\n", "\n      "))
		}
	}
	waitKey("")
}

func handleGGUFInspect() {
	fmt.Print("\033[H\033[2J")
	fmt.Println("\n  🔍 GGUF Inspect")
	fmt.Println()
	fmt.Print("  GGUF file path: ")
	path := strings.TrimSpace(readLineSimple())
	if path == "" {
		return
	}

	result, err := apiCall("POST", "/api/gguf/inspect", map[string]interface{}{"path": path})
	if err != nil {
		waitKey(fmt.Sprintf("  ❌  %s", err.Error()))
		return
	}
	if errMsg, ok := result["error"].(string); ok {
		waitKey(fmt.Sprintf("  ❌  %s", errMsg))
		return
	}

	fmt.Print("\033[H\033[2J")
	fmt.Println("\n  🔍 GGUF Metadata")
	fmt.Println()
	for k, v := range result {
		val := fmt.Sprintf("%v", v)
		if len(val) > 60 {
			val = val[:57] + "…"
		}
		fmt.Printf("  %-30s  %s\n", k, val)
	}
	waitKey("")
}

func handleAuditLog() {
	result, err := apiCall("GET", "/api/audit?limit=20", nil)
	if err != nil {
		waitKey(fmt.Sprintf("  ❌  %s", err.Error()))
		return
	}
	if errMsg, ok := result["error"].(string); ok {
		waitKey(fmt.Sprintf("  ❌  %s", errMsg))
		return
	}

	fmt.Print("\033[H\033[2J")
	fmt.Println("\n  📋 Audit Log (last 20)")
	fmt.Println()
	entries, _ := result["entries"].([]interface{})
	if len(entries) == 0 {
		waitKey("  No audit entries.")
		return
	}
	for i := len(entries) - 1; i >= 0; i-- {
		if e, ok := entries[i].(map[string]interface{}); ok {
			ts, _ := e["timestamp"].(string)
			if len(ts) > 19 {
				ts = ts[:19]
			}
			method, _ := e["method"].(string)
			path, _ := e["path"].(string)
			status := fmt.Sprintf("%.0f", e["status"])
			dur := fmt.Sprintf("%.0fms", e["duration_ms"])
			fmt.Printf("  %s  %-6s %-35s  %s  %s\n", ts, method, path, status, dur)
		}
	}
	waitKey("")
}

func readLineSimple() string {
	var line strings.Builder
	buf := make([]byte, 1)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			break
		}
		c := buf[0]
		if c == '\n' || c == '\r' {
			break
		}
		if c == 8 || c == 127 { // backspace
			s := line.String()
			if len(s) > 0 {
				line.Reset()
				line.WriteString(s[:len(s)-1])
				fmt.Print("\b \b")
			}
			continue
		}
		line.WriteByte(c)
		fmt.Printf("%c", c)
	}
	fmt.Println()
	return line.String()
}

// ─── helper: message + wait for key ─────────────────────────────────────────

func waitKey(msg string) {
	fmt.Print("\033[H\033[2J")
	fmt.Println()
	fmt.Println(msg)
	fmt.Println()
	fmt.Print("  \033[2mPress any key to continue…\033[0m")

	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err == nil {
		buf := make([]byte, 1)
		os.Stdin.Read(buf)
		term.Restore(fd, old)
	}
}

// ─── selectMenu ──────────────────────────────────────────────────────────────
//
// Shows an interactive menu and returns the selected index, or -1
// if the user presses Esc or Ctrl-C.
//
// Layout (N = len(items), final cursor = end of hint line, no \n):
//
//	[blank]
//	  <title>
//	[blank]
//	  item 0          ← N+1 lines above cursor
//	  item 1
//	  …
//	  item N-1
//	[blank]
//	  ↑/↓  enter  esc ← cursor here
func selectMenu(title string, items []string) int {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return -1
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return -1
	}
	defer term.Restore(fd, old)

	sel := 0
	menuDrawFull(title, items, sel)

	buf := make([]byte, 4)
	for {
		n, _ := os.Stdin.Read(buf)
		if n == 0 {
			break
		}
		b := buf[:n]
		switch {
		case b[0] == 27 && n == 1, b[0] == 3: // esc / ctrl-c
			fmt.Print("\033[H\033[2J")
			return -1
		case n >= 3 && b[0] == 27 && b[1] == '[' && b[2] == 'A': // up
			if sel > 0 {
				sel--
				menuDrawItems(items, sel)
			}
		case n >= 3 && b[0] == 27 && b[1] == '[' && b[2] == 'B': // down
			if sel < len(items)-1 {
				sel++
				menuDrawItems(items, sel)
			}
		case b[0] == 13 || b[0] == 10: // enter
			fmt.Print("\033[H\033[2J")
			return sel
		}
	}
	return -1
}

func menuDrawFull(title string, items []string, sel int) {
	fmt.Print("\033[H\033[2J")
	fmt.Println()
	fmt.Printf("  %s\n", title)
	fmt.Println()
	for i, item := range items {
		if i == sel {
			fmt.Printf("  \033[1m> %s\033[0m\n", item)
		} else {
			fmt.Printf("    %s\n", item)
		}
	}
	fmt.Println()
	fmt.Print("  \033[2m↑/↓  enter  esc\033[0m")
}

// menuDrawItems updates only the items in-place (no full-screen clear).
// It moves up len(items)+1 lines (items + final blank line), rewrites everything
// up to the hint line, and leaves the cursor where it was.
func menuDrawItems(items []string, sel int) {
	fmt.Printf("\033[%dA\r", len(items)+1)
	for i, item := range items {
		fmt.Print("\033[2K")
		if i == sel {
			fmt.Printf("  \033[1m> %s\033[0m\n", item)
		} else {
			fmt.Printf("    %s\n", item)
		}
	}
	fmt.Print("\033[2K\n\033[2K  \033[2m↑/↓  enter  esc\033[0m")
}

// ─── reExec ──────────────────────────────────────────────────────────────────

func reExec(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	proc, err := os.StartProcess(exe, append([]string{exe}, args...), &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return err
	}
	s, err := proc.Wait()
	if err != nil {
		return err
	}
	if !s.Success() {
		os.Exit(s.ExitCode())
	}
	return nil
}
