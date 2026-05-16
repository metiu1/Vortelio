package hub

import (
	"testing"
)

func TestParseModelRef_standard(t *testing.T) {
	cases := []struct {
		input    string
		wantType string
		wantName string
		wantTag  string
		wantErr  bool
	}{
		{"llm/mistral:7b", "llm", "mistral", "7b", false},
		{"image/sdxl:latest", "image", "sdxl", "latest", false},
		{"llm/phi3:mini", "llm", "phi3", "mini", false},
		{"audio/whisper:large", "audio", "whisper", "large", false},
		{"video/cogvideo", "video", "cogvideo", "latest", false},
		{"3d/shap-e", "3d", "shap-e", "latest", false},
		// Missing type prefix
		{"mistral:7b", "", "", "", true},
		// Unknown type
		{"unknown/model:tag", "", "", "", true},
		// Local file extension — should error
		{"image/model.gguf", "", "", "", true},
	}
	for _, c := range cases {
		ref, err := ParseModelRef(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseModelRef(%q): want error, got nil", c.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseModelRef(%q): unexpected error: %v", c.input, err)
			continue
		}
		if ref.Type != c.wantType { t.Errorf("ParseModelRef(%q).Type = %q, want %q", c.input, ref.Type, c.wantType) }
		if ref.Name != c.wantName { t.Errorf("ParseModelRef(%q).Name = %q, want %q", c.input, ref.Name, c.wantName) }
		if ref.Tag != c.wantTag  { t.Errorf("ParseModelRef(%q).Tag  = %q, want %q", c.input, ref.Tag, c.wantTag) }
	}
}

func TestParseModelRef_hfdirect(t *testing.T) {
	cases := []struct {
		input     string
		wantType  string
		wantOwner string
		wantRepo  string
		wantHint  string
	}{
		{
			"llm/hf.co/unsloth/Qwen3.5-0.8B-GGUF:UD-IQ2_XXS",
			"llm", "unsloth", "Qwen3.5-0.8B-GGUF", "UD-IQ2_XXS",
		},
		{
			"llm/hf.co/bartowski/Mistral-7B-v0.1-GGUF:Q4_K_M",
			"llm", "bartowski", "Mistral-7B-v0.1-GGUF", "Q4_K_M",
		},
	}
	for _, c := range cases {
		ref, err := ParseModelRef(c.input)
		if err != nil {
			t.Errorf("ParseModelRef(%q): unexpected error: %v", c.input, err)
			continue
		}
		if ref.HFDirect == nil {
			t.Errorf("ParseModelRef(%q): HFDirect is nil", c.input)
			continue
		}
		if ref.Type != c.wantType       { t.Errorf("%q Type = %q, want %q", c.input, ref.Type, c.wantType) }
		if ref.HFDirect.Owner != c.wantOwner { t.Errorf("%q Owner = %q, want %q", c.input, ref.HFDirect.Owner, c.wantOwner) }
		if ref.HFDirect.Repo != c.wantRepo   { t.Errorf("%q Repo = %q, want %q", c.input, ref.HFDirect.Repo, c.wantRepo) }
		if ref.HFDirect.FileHint != c.wantHint { t.Errorf("%q FileHint = %q, want %q", c.input, ref.HFDirect.FileHint, c.wantHint) }
	}
}

func TestParseModelRef_hfurl(t *testing.T) {
	cases := []struct {
		input     string
		wantType  string
		wantOwner string
		wantRepo  string
	}{
		{
			"llm/https://huggingface.co/unsloth/Qwen3.5-0.8B-GGUF",
			"llm", "unsloth", "Qwen3.5-0.8B-GGUF",
		},
	}
	for _, c := range cases {
		ref, err := ParseModelRef(c.input)
		if err != nil {
			t.Errorf("ParseModelRef(%q): unexpected error: %v", c.input, err)
			continue
		}
		if ref.HFDirect == nil {
			t.Errorf("ParseModelRef(%q): HFDirect is nil", c.input)
			continue
		}
		if ref.Type != c.wantType             { t.Errorf("%q Type  = %q, want %q", c.input, ref.Type, c.wantType) }
		if ref.HFDirect.Owner != c.wantOwner  { t.Errorf("%q Owner = %q, want %q", c.input, ref.HFDirect.Owner, c.wantOwner) }
		if ref.HFDirect.Repo != c.wantRepo    { t.Errorf("%q Repo  = %q, want %q", c.input, ref.HFDirect.Repo, c.wantRepo) }
	}
}

func TestModelRef_String(t *testing.T) {
	ref := &ModelRef{Type: "llm", Name: "mistral", Tag: "7b"}
	want := "llm/mistral:7b"
	if got := ref.String(); got != want {
		t.Errorf("ModelRef.String() = %q, want %q", got, want)
	}
}
