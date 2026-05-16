package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

type ImportOllamaCommand struct{}

func NewImportOllamaCommand() *ImportOllamaCommand { return &ImportOllamaCommand{} }
func (c *ImportOllamaCommand) Name() string         { return "import-ollama" }

func (c *ImportOllamaCommand) Run(args []string) error {
	dryRun := false
	customPath := ""
	for i, a := range args {
		switch a {
		case "--dry-run", "-n":
			dryRun = true
		case "--path":
			if i+1 < len(args) {
				customPath = args[i+1]
			}
		}
	}

	// Try direct local path first (no server needed)
	if customPath == "" {
		customPath = defaultOllamaPath()
	}
	fmt.Printf("📦 Importing Ollama models from: %s\n", customPath)
	if dryRun {
		fmt.Println("    (dry run — no changes)")
	}

	// Try the running server first; fall back to direct stdin approach
	port := fmt.Sprintf("%d", config.Get().Port)
	body, _ := json.Marshal(map[string]interface{}{
		"ollama_path": customPath,
		"dry_run":     dryRun,
	})
	resp, err := http.Post("http://127.0.0.1:"+port+"/api/import/ollama", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("server non in esecuzione — avvia 'vortelio serve' prima: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var er map[string]interface{}
		json.Unmarshal(data, &er)
		return fmt.Errorf("server error %d: %v", resp.StatusCode, er["error"])
	}

	var result struct {
		Imported []map[string]interface{} `json:"imported"`
		Skipped  []map[string]interface{} `json:"skipped"`
		Count    int                      `json:"count"`
	}
	json.Unmarshal(data, &result)

	for _, it := range result.Imported {
		mark := "✅"
		if dryRun {
			mark = "🔎"
		}
		fmt.Printf("    %s %s\n", mark, it["model"])
	}
	for _, it := range result.Skipped {
		fmt.Printf("    ⏭  %s — %s\n", it["model"], it["reason"])
	}
	fmt.Printf("\n%d modelli importati.\n", result.Count)
	return nil
}

func defaultOllamaPath() string {
	if v := os.Getenv("OLLAMA_MODELS"); v != "" {
		return filepath.Dir(filepath.Dir(v))
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ollama")
}

// Touch time to avoid unused import on some platforms
var _ = time.Now
