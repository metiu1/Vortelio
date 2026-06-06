package cloud

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Verifies the Gemini fix: system messages go into systemInstruction (not a
// synthetic user turn), contents stay strictly alternating user/model, and the
// tool loop sends a functionResponse back after executing a functionCall.
func TestGeminiSystemInstructionAndToolLoop(t *testing.T) {
	var bodies []map[string]interface{}
	round := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]interface{}
		json.Unmarshal(raw, &b)
		bodies = append(bodies, b)
		w.Header().Set("Content-Type", "application/json")
		if round == 0 {
			round++
			// First reply: a function call to "add".
			io.WriteString(w, `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"add","args":{"a":2,"b":3}}}]},"finishReason":"STOP"}]}`)
			return
		}
		// Second reply: final text.
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"The sum is 5."}]},"finishReason":"STOP"}]}`)
	}))
	defer srv.Close()

	p := Provider{ID: "gemini", Format: FormatGemini, BaseURL: srv.URL}
	msgs := []Message{
		{Role: "system", Content: "You are a precise assistant."},
		{Role: "user", Content: "What is 2 plus 3?"},
	}
	var executed string
	opts := &ToolCallOptions{
		Tools: []map[string]interface{}{{
			"type": "function",
			"function": map[string]interface{}{
				"name":        "add",
				"description": "add two numbers",
				"parameters":  map[string]interface{}{"type": "object"},
			},
		}},
		ExecTool: func(name, args string) (string, error) {
			executed = name + " " + args
			return `{"sum":5}`, nil
		},
	}

	out, err := chatGeminiWithTools(p, "fake-key", msgs, opts, nil)
	if err != nil {
		t.Fatalf("chatGeminiWithTools error: %v", err)
	}
	if !strings.Contains(out, "The sum is 5") {
		t.Fatalf("unexpected final output: %q", out)
	}
	if !strings.Contains(executed, "add") {
		t.Fatalf("tool was not executed, got %q", executed)
	}

	// Round 1 body: systemInstruction present, contents has exactly one user turn.
	b0 := bodies[0]
	si, ok := b0["systemInstruction"].(map[string]interface{})
	if !ok {
		t.Fatalf("round 0 missing systemInstruction; body=%v", b0)
	}
	if !strings.Contains(toJSON(si), "precise assistant") {
		t.Fatalf("systemInstruction missing system text: %v", si)
	}
	contents0 := b0["contents"].([]interface{})
	if len(contents0) != 1 {
		t.Fatalf("round 0 expected 1 content (user only), got %d: %v", len(contents0), contents0)
	}
	if role := contents0[0].(map[string]interface{})["role"]; role != "user" {
		t.Fatalf("round 0 first content role = %v, want user", role)
	}

	// Round 2 body: contents must alternate user, model(functionCall), user(functionResponse).
	b1 := bodies[1]
	contents1 := b1["contents"].([]interface{})
	roles := []string{}
	for _, c := range contents1 {
		roles = append(roles, c.(map[string]interface{})["role"].(string))
	}
	want := []string{"user", "model", "user"}
	if strings.Join(roles, ",") != strings.Join(want, ",") {
		t.Fatalf("round 1 roles = %v, want %v", roles, want)
	}
	if !strings.Contains(toJSON(contents1), "functionResponse") {
		t.Fatalf("round 1 missing functionResponse: %v", contents1)
	}
}

func toJSON(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
