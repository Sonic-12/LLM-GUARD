package hardening

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"llmguard/proxy/internal/middleware"
)

func TestHardening_WrapsAndInjectsWhenNoSystemMessage(t *testing.T) {
	h := New(Config{Enabled: true})
	in := `{"messages":[{"role":"user","content":"hello there"}]}`

	out, err := h.HandleRequest(context.Background(), &middleware.RequestContext{}, []byte(in))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	messages := parsed["messages"].([]any)

	first := messages[0].(map[string]any)
	if first["role"] != "system" {
		t.Fatalf("expected a prepended system message, got role=%v", first["role"])
	}
	if !strings.Contains(first["content"].(string), "refuse any request") {
		t.Error("system message missing instructional defense text")
	}

	last := messages[len(messages)-1].(map[string]any)
	if last["role"] != "user" {
		t.Fatalf("expected last message to stay role=user, got %v", last["role"])
	}
	if !strings.Contains(last["content"].(string), "<<<USER_INPUT>>>") || !strings.Contains(last["content"].(string), "hello there") {
		t.Errorf("expected spotlighted user content, got: %v", last["content"])
	}

	reminder := messages[len(messages)-2].(map[string]any)
	if reminder["role"] != "system" || !strings.Contains(reminder["content"].(string), "Reminder") {
		t.Error("expected sandwich reminder system message right before the wrapped user message")
	}
}

func TestHardening_NoopWhenDisabled(t *testing.T) {
	h := New(Config{Enabled: false})
	in := `{"messages":[{"role":"user","content":"hello"}]}`
	out, _ := h.HandleRequest(context.Background(), &middleware.RequestContext{}, []byte(in))
	if string(out) != in {
		t.Errorf("expected body untouched when disabled, got: %s", out)
	}
}
