package rbac

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"llmguard/proxy/internal/middleware"
)

const testSecret = "test-secret-do-not-use-in-prod"

func testConfig() Config {
	return Config{
		Enabled:      true,
		Secret:       testSecret,
		DefaultModel: "llama3.2:3b",
		PremiumModel: "llama3.1:8b",
		Roles: map[string]RoleConfig{
			"admin":    {AllowedModels: map[string]bool{"default": true, "premium": true}, MaxChars: 4000},
			"employee": {AllowedModels: map[string]bool{"default": true, "premium": true}, MaxChars: 4000},
			"guest":    {AllowedModels: map[string]bool{"default": true}, MaxChars: 1000},
		},
	}
}

func chatBody(model, content string) []byte {
	body, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": []map[string]any{{"role": "user", "content": content}},
	})
	return body
}

func newRC(header string) *middleware.RequestContext {
	return &middleware.RequestContext{
		RequestID: "test",
		Metadata:  map[string]any{MetadataAuthHeader: header},
	}
}

func TestHandleRequest_MissingToken(t *testing.T) {
	hook := New(testConfig())
	_, err := hook.HandleRequest(context.Background(), newRC(""), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_InvalidToken(t *testing.T) {
	hook := New(testConfig())
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer not-a-real-token"), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_ExpiredToken(t *testing.T) {
	hook := New(testConfig())
	tok, _ := NewToken(testSecret, "u1", "admin", -1*time.Minute)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 401)
}

func TestHandleRequest_UnknownRole(t *testing.T) {
	hook := New(testConfig())
	tok, _ := NewToken(testSecret, "u1", "superadmin", time.Hour)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 403)
}

func TestHandleRequest_GuestBlockedFromPremiumModel(t *testing.T) {
	hook := New(testConfig())
	tok, _ := NewToken(testSecret, "u1", "guest", time.Hour)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.1:8b", "hi"))
	assertBlockedStatus(t, err, 403)
}

func TestHandleRequest_GuestOverRoleCharLimit(t *testing.T) {
	hook := New(testConfig())
	tok, _ := NewToken(testSecret, "u1", "guest", time.Hour)
	longPrompt := make([]byte, 1500)
	for i := range longPrompt {
		longPrompt[i] = 'a'
	}
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer "+tok), chatBody("llama3.2:3b", string(longPrompt)))
	assertBlockedStatus(t, err, 403)
}

func TestHandleRequest_AdminAllowedThrough(t *testing.T) {
	hook := New(testConfig())
	tok, _ := NewToken(testSecret, "u1", "admin", time.Hour)
	rc := newRC("Bearer " + tok)
	out, err := hook.HandleRequest(context.Background(), rc, chatBody("llama3.1:8b", "hi"))
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
	if rc.UserID != "u1" {
		t.Errorf("expected rc.UserID to be set, got %q", rc.UserID)
	}
	if rc.Metadata[MetadataRole] != "admin" {
		t.Errorf("expected rc.Metadata role to be admin, got %v", rc.Metadata[MetadataRole])
	}
}

func TestHandleRequest_DisabledPassesThrough(t *testing.T) {
	cfg := testConfig()
	cfg.Enabled = false
	hook := New(cfg)
	out, err := hook.HandleRequest(context.Background(), newRC(""), chatBody("llama3.2:3b", "hi"))
	if err != nil {
		t.Fatalf("expected disabled hook to pass through, got error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
}

func TestHandleRequest_MissingSecretFailsClosed(t *testing.T) {
	cfg := testConfig()
	cfg.Secret = ""
	hook := New(cfg)
	_, err := hook.HandleRequest(context.Background(), newRC("Bearer whatever"), chatBody("llama3.2:3b", "hi"))
	assertBlockedStatus(t, err, 503)
}

func assertBlockedStatus(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected request to be blocked, got nil error")
	}
	blocked, ok := err.(*BlockedError)
	if !ok {
		t.Fatalf("expected *BlockedError, got %T: %v", err, err)
	}
	if blocked.StatusCode != want {
		t.Errorf("expected status %d, got %d", want, blocked.StatusCode)
	}
}