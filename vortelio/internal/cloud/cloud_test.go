package cloud

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadKey(t *testing.T) {
	// Use a temp directory to avoid touching real ~/.vortelio
	tmp := t.TempDir()
	t.Setenv("VORTELIO_HOME", tmp)

	const (
		provider = "openai"
		key      = "sk-test-1234567890abcdef"
	)

	if err := SaveKey(provider, key); err != nil {
		t.Fatalf("SaveKey: %v", err)
	}

	// File must exist
	if _, err := os.Stat(filepath.Join(tmp, "cloud_keys.json")); err != nil {
		t.Fatalf("cloud_keys.json not created: %v", err)
	}

	got := LoadKey(provider)
	if got != key {
		t.Errorf("LoadKey(%q) = %q, want %q", provider, got, key)
	}
}

func TestLoadKey_missing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VORTELIO_HOME", tmp)

	got := LoadKey("nonexistent_provider")
	if got != "" {
		t.Errorf("LoadKey for missing provider = %q, want empty string", got)
	}
}

func TestSaveKey_multiple(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VORTELIO_HOME", tmp)

	keys := map[string]string{
		"openai":    "sk-openai-key",
		"anthropic": "sk-ant-anthropic-key",
		"gemini":    "AIza-gemini-key",
	}

	for provider, key := range keys {
		if err := SaveKey(provider, key); err != nil {
			t.Fatalf("SaveKey(%q): %v", provider, err)
		}
	}

	for provider, want := range keys {
		got := LoadKey(provider)
		if got != want {
			t.Errorf("LoadKey(%q) = %q, want %q", provider, got, want)
		}
	}
}

func TestSaveKey_overwrite(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VORTELIO_HOME", tmp)

	if err := SaveKey("openai", "sk-old"); err != nil { t.Fatal(err) }
	if err := SaveKey("openai", "sk-new"); err != nil { t.Fatal(err) }

	got := LoadKey("openai")
	if got != "sk-new" {
		t.Errorf("LoadKey after overwrite = %q, want %q", got, "sk-new")
	}
}

func TestFindProvider(t *testing.T) {
	for _, id := range []string{"openai", "anthropic", "gemini", "groq", "mistral", "openrouter"} {
		p, ok := FindProvider(id)
		if !ok {
			t.Errorf("FindProvider(%q): not found", id)
			continue
		}
		if p.ID != id {
			t.Errorf("FindProvider(%q).ID = %q", id, p.ID)
		}
		if p.BaseURL == "" {
			t.Errorf("FindProvider(%q).BaseURL is empty", id)
		}
	}

	_, ok := FindProvider("nonexistent")
	if ok {
		t.Error("FindProvider(nonexistent): expected false, got true")
	}
}
