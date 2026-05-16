package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetSingleton() {
	once = sync.Once{}
	instance = nil
}

func TestHomeDir_default(t *testing.T) {
	os.Unsetenv("VORTELIO_HOME")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".vortelio")
	if got := HomeDir(); got != want {
		t.Errorf("HomeDir() = %q, want %q", got, want)
	}
}

func TestHomeDir_env(t *testing.T) {
	t.Setenv("VORTELIO_HOME", "/custom/path")
	if got := HomeDir(); got != "/custom/path" {
		t.Errorf("HomeDir() with env = %q, want /custom/path", got)
	}
}

func TestLoad_defaults(t *testing.T) {
	resetSingleton()
	os.Unsetenv("VORTELIO_HOME")

	tmp := t.TempDir()
	t.Setenv("VORTELIO_HOME", tmp)
	resetSingleton()

	cfg := Load()
	if cfg.Port != 11500          { t.Errorf("default Port = %d, want 11500", cfg.Port) }
	if cfg.OllamaPort != 11434    { t.Errorf("default OllamaPort = %d, want 11434", cfg.OllamaPort) }
	if cfg.CloudTimeoutSec != 120 { t.Errorf("default CloudTimeoutSec = %d, want 120", cfg.CloudTimeoutSec) }
}

func TestLoad_fromFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VORTELIO_HOME", tmp)
	resetSingleton()

	cfgJSON := `{"port": 9999, "ollama_port": 12345, "cloud_timeout_sec": 60}`
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), []byte(cfgJSON), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if cfg.Port != 9999           { t.Errorf("Port = %d, want 9999", cfg.Port) }
	if cfg.OllamaPort != 12345    { t.Errorf("OllamaPort = %d, want 12345", cfg.OllamaPort) }
	if cfg.CloudTimeoutSec != 60  { t.Errorf("CloudTimeoutSec = %d, want 60", cfg.CloudTimeoutSec) }
}
