package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/vortelio/vortelio/internal/agent"
	"github.com/vortelio/vortelio/internal/config"
)

func handleAgentiAI() error {
	for {
		sel := selectMenu("AI Agents", agentMenuLabels())
		if sel < 0 || sel == len(agent.Catalog) {
			return nil
		}
		entry := agent.Catalog[sel]
		if err := handleAgentDetail(entry); err != nil {
			return err
		}
	}
}

func agentMenuLabels() []string {
	labels := make([]string, len(agent.Catalog)+1)
	for i, e := range agent.Catalog {
		state := agent.GetState(e.ID)
		var badge string
		switch {
		case state.Running:
			badge = "● running"
		case state.Installed:
			badge = "📦 installed"
		default:
			badge = "   not installed"
		}
		labels[i] = fmt.Sprintf("%-18s  %s", e.Name, badge)
	}
	labels[len(agent.Catalog)] = "← Back"
	return labels
}

func handleCrewAI() error {
	crewEntry, ok := func() (agent.CatalogEntry, bool) {
		for _, e := range agent.Catalog {
			if e.ID == "crewai" {
				return e, true
			}
		}
		return agent.CatalogEntry{}, false
	}()
	if !ok {
		waitKey("  ❌  CrewAI non trovato nel catalogo.")
		return nil
	}

	for {
		state := agent.GetState("crewai")

		var items []string
		if state.Running {
			items = append(items, "🤖  Apri gestione Crew (Web GUI)")
			items = append(items, "⏹  Stop CrewAI server")
		} else if state.Installed {
			items = append(items, "▶  Avvia CrewAI server (porta 8500)")
			items = append(items, "🗑  Disinstalla CrewAI")
		} else {
			if state.PipFound {
				items = append(items, "⬇  Installa CrewAI (pip)")
			} else {
				items = append(items, "⚠  Python/pip non trovato")
			}
		}
		items = append(items, "← Back")

		sel := selectMenu("🤖 CrewAI Orchestration", items)
		if sel < 0 {
			return nil
		}
		chosen := items[sel]

		switch {
		case chosen == "← Back" || strings.HasPrefix(chosen, "⚠"):
			return nil
		case strings.HasPrefix(chosen, "▶"):
			if err := agent.Start("crewai"); err != nil {
				waitKey(fmt.Sprintf("  ❌  Avvio fallito: %s", err.Error()))
			} else {
				waitKey("  ✅  CrewAI server avviato su http://localhost:8500\n  Apri la Web GUI per gestire le crew.")
			}
		case strings.HasPrefix(chosen, "⏹"):
			agent.Stop("crewai")
			waitKey("  ✅  CrewAI server fermato.")
		case strings.HasPrefix(chosen, "🤖"):
			vortURL := fmt.Sprintf("http://localhost:%d", config.Get().Port)
			openBrowser(vortURL)
			waitKey("  🌐  Aperta Web GUI. Vai su 🤖 CrewAI nel menu laterale.")
		case strings.HasPrefix(chosen, "⬇"):
			if err := runAgentInstall(crewEntry); err != nil {
				waitKey(fmt.Sprintf("  ❌  Installazione fallita: %s", err.Error()))
			}
		case strings.HasPrefix(chosen, "🗑"):
			confirm := selectMenu("Conferma disinstallazione CrewAI", []string{"Sì, disinstalla", "Annulla"})
			if confirm == 0 {
				if err := agent.Uninstall("crewai"); err != nil {
					waitKey(fmt.Sprintf("  ❌  Disinstallazione fallita: %s", err.Error()))
				} else {
					waitKey("  ✅  CrewAI disinstallato.")
				}
			}
		}
	}
}

func handleAgentDetail(entry agent.CatalogEntry) error {
	for {
		state := agent.GetState(entry.ID)

		// Build context-sensitive action list
		var actions []string
		if state.Running {
			actions = append(actions, "⏹  Stop agent")
			if entry.DefaultURL != "" {
				actions = append(actions, "🌐  Open in browser")
			}
			if entry.ID == "crewai" {
				actions = append(actions, "🤖  Gestisci Crew (Web GUI)")
			}
		} else if state.Installed {
			actions = append(actions, "▶  Start agent")
			actions = append(actions, "🗑  Uninstall agent")
		} else {
			canInstall := false
			var missingMsg string
			switch entry.InstallMethod {
			case agent.MethodPip:
				canInstall = state.PipFound
				missingMsg = "⚠  Python/pip not found (install from python.org)"
			default:
				canInstall = state.NodeFound
				missingMsg = "⚠  Node.js not found (install from nodejs.org)"
			}
			if canInstall {
				actions = append(actions, "⬇  Install agent")
			} else {
				actions = append(actions, missingMsg)
			}
		}
		actions = append(actions, "← Back")

		// Truncate description for title
		desc := entry.Description
		if len(desc) > 50 {
			desc = desc[:47] + "…"
		}
		sel := selectMenu(entry.Name+" — "+desc, actions)
		if sel < 0 {
			return nil
		}

		chosen := actions[sel]
		switch {
		case chosen == "← Back" || strings.HasPrefix(chosen, "⚠"):
			return nil

		case strings.HasPrefix(chosen, "▶"):
			if err := agent.Start(entry.ID); err != nil {
				waitKey(fmt.Sprintf("  ❌  Start failed:\n  %s", err.Error()))
			} else {
				waitKey(fmt.Sprintf("  ✅  %s started at %s", entry.Name, entry.DefaultURL))
			}

		case strings.HasPrefix(chosen, "⏹"):
			agent.Stop(entry.ID)
			waitKey(fmt.Sprintf("  ✅  %s stopped.", entry.Name))

		case strings.HasPrefix(chosen, "🌐"):
			openBrowser(entry.DefaultURL)
			waitKey(fmt.Sprintf("  🌐  Opening %s in browser…", entry.DefaultURL))

		case strings.HasPrefix(chosen, "🤖"):
			vortURL := fmt.Sprintf("http://localhost:%d", config.Get().Port)
			openBrowser(vortURL)
			waitKey("  🌐  Aperta Web GUI. Vai su 🤖 CrewAI nel menu laterale.")

		case strings.HasPrefix(chosen, "⬇"):
			if err := runAgentInstall(entry); err != nil {
				waitKey(fmt.Sprintf("  ❌  Installation failed:\n  %s", err.Error()))
			}

		case strings.HasPrefix(chosen, "🗑"):
			confirm := selectMenu(
				"Confirm uninstall of "+entry.Name,
				[]string{"Yes, uninstall", "Cancel"},
			)
			if confirm == 0 {
				if err := agent.Uninstall(entry.ID); err != nil {
					waitKey(fmt.Sprintf("  ❌  Uninstall failed: %s", err.Error()))
				} else {
					waitKey(fmt.Sprintf("  ✅  %s uninstalled.", entry.Name))
				}
			}
		}
	}
}

// runAgentInstall streams npm install output to the terminal.
func runAgentInstall(entry agent.CatalogEntry) error {
	fmt.Print("\033[H\033[2J")
	fmt.Printf("\n  ⬇  Installing %s…\n\n", entry.Name)

	var lastErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx := context.Background()
		lastErr = agent.Install(ctx, entry.ID, func(line string) {
			fmt.Printf("  %s\n", line)
		})
	}()
	<-done

	if lastErr != nil {
		return lastErr
	}
	fmt.Printf("\n  ✅  Installation complete!\n")
	waitKey("")
	return nil
}

// openBrowser opens a URL in the default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Start()
}

// waitEnter: during install we are not in raw mode, so we wait for \n.
func waitEnter() {
	fmt.Print("\n  Press Enter to continue… ")
	buf := make([]byte, 1)
	for {
		os.Stdin.Read(buf)
		if buf[0] == '\n' || buf[0] == '\r' {
			break
		}
	}
}
