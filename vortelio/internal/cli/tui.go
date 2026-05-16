package cli

import (
	"fmt"
	"os"

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
			return reExec("gui")
		case 5:
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
