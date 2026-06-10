package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/vortelio/vortelio/internal/cloud"
	"github.com/vortelio/vortelio/internal/runtime"
	"golang.org/x/term"
)

func handleModelloCloud() error {
	// Build provider labels
	labels := make([]string, len(cloud.Providers)+1)
	for i, p := range cloud.Providers {
		labels[i] = p.Name
	}
	labels[len(cloud.Providers)] = "← Back"

	sel := selectMenu("Cloud Models", labels)
	if sel < 0 || sel == len(cloud.Providers) {
		return nil
	}

	p := cloud.Providers[sel]
	return runCloudChat(p)
}

func runCloudChat(p cloud.Provider) error {
	// Ensure terminal is in normal (non-raw) mode for text input
	fmt.Print("\033[H\033[2J")
	fmt.Printf("\n  %s\n", p.Name)
	fmt.Printf("  Model: %s\n\n", p.DefaultModel)

	// Load or ask for API key
	apiKey := cloud.LoadKey(p.ID)
	if apiKey == "" {
		fmt.Printf("  No API key found for %s.\n", p.Name)
		fmt.Printf("  Get your key at: %s\n\n", p.KeyHint)
		fmt.Print("  Paste your API key: ")
		apiKey = readLine()
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			waitKey("  ❌  No API key entered. Operation cancelled.")
			return nil
		}
		if err := cloud.SaveKey(p.ID, apiKey); err != nil {
			fmt.Printf("  ⚠  Could not save key: %s\n", err)
		} else {
			fmt.Printf("  ✅  Key saved.\n\n")
		}
	} else {
		masked := maskKey(apiKey)
		fmt.Printf("  API key: %s  (saved)\n", masked)
		fmt.Printf("  [Press Enter to use it, or type a new key]: ")
		line := readLine()
		line = strings.TrimSpace(line)
		if line != "" {
			apiKey = line
			if err := cloud.SaveKey(p.ID, apiKey); err != nil {
				fmt.Printf("  ⚠  Could not save key: %s\n", err)
			} else {
				fmt.Printf("  ✅  New key saved.\n\n")
			}
		}
	}

	// Chat loop. Use all stored keys for failover (the active key is saved above
	// so it's first); fall back to the just-entered one if nothing is stored.
	keys := cloud.LoadKeys(p.ID)
	if len(keys) == 0 {
		keys = []string{apiKey}
	}
	var history []cloud.Message
	fmt.Printf("\n  Chat with %s  (type 'exit' to quit)\n", p.Name)
	fmt.Println("  " + strings.Repeat("─", 50))

	for {
		fmt.Print("\n  You: ")
		input := readLine()
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
			break
		}

		history = append(history, cloud.Message{Role: "user", Content: input})

		fmt.Printf("\n  %s:\n  ", p.Name)

		toolOpts := &cloud.ToolCallOptions{
			Tools:    runtime.BuiltinTools(),
			ExecTool: runtime.ExecuteTool,
		}
		response, err := cloud.ChatWithToolsFailover(p, keys, history, toolOpts, func(tok string) {
			tok = strings.ReplaceAll(tok, "\n", "\n  ")
			fmt.Print(tok)
		})
		fmt.Println()

		if err != nil {
			fmt.Printf("\n  ❌  Error: %s\n", err)
			// Remove the user message we added since the request failed
			history = history[:len(history)-1]
			fmt.Print("\n  Retry? (enter=yes / exit=no): ")
			ans := readLine()
			if strings.ToLower(strings.TrimSpace(ans)) == "exit" {
				break
			}
			continue
		}

		history = append(history, cloud.Message{Role: "assistant", Content: response})
		fmt.Println("  " + strings.Repeat("─", 50))
	}

	waitKey(fmt.Sprintf("  Chat with %s ended.", p.Name))
	return nil
}

// stdinReader is a single shared buffered reader. Allocating a new
// bufio.Scanner per readLine() call (the previous approach) discarded any bytes
// it buffered past the newline, losing input on multi-line paste. One shared
// reader keeps that remainder for the next call.
var stdinReader = bufio.NewReader(os.Stdin)

// readLine reads a full line in normal (non-raw) terminal mode.
func readLine() string {
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		// Restore normal mode temporarily (it may already be normal, that's fine)
		old, err := term.GetState(fd)
		if err == nil {
			term.Restore(fd, old)
		}
	}
	line, err := stdinReader.ReadString('\n')
	if line == "" && err != nil {
		return ""
	}
	return strings.TrimRight(line, "\r\n")
}

// maskKey shows only the first 4 and last 4 chars.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
