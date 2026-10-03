package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogBlocked_WritesLocalJSONL(t *testing.T) {
	path := t.TempDir() + "/events.jsonl"
	l := New(Config{Enabled: true, LocalLogPath: path})
	defer l.Close()

	l.LogBlocked(AuditEvent{
		UserID:       "admin-user",
		Source:       "rbac",
		Allowed:      false,
		StatusCode:   403,
		Reason:       "MODEL_NOT_ALLOWED_FOR_ROLE",
		PromptLength: 5,
		PromptSample: "hello",
	})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	line := strings.TrimSpace(string(data))

	var got AuditEvent
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("log line is not valid JSON: %v (line=%q)", err, line)
	}
	if got.UserID != "admin-user" || got.Source != "rbac" || got.StatusCode != 403 {
		t.Errorf("unexpected event content: %+v", got)
	}
	if got.EventID == "" {
		t.Error("expected an auto-generated event_id, got empty string")
	}
	if got.Timestamp == "" {
		t.Error("expected an auto-generated timestamp, got empty string")
	}
}

func TestLogBlocked_TruncatesLongPromptSample(t *testing.T) {
	path := t.TempDir() + "/events.jsonl"
	l := New(Config{Enabled: true, LocalLogPath: path})
	defer l.Close()

	longPrompt := strings.Repeat("a", 500)
	l.LogBlocked(AuditEvent{PromptSample: longPrompt})

	data, _ := os.ReadFile(path)
	var got AuditEvent
	json.Unmarshal(data, &got)

	if !strings.HasSuffix(got.PromptSample, "...[TRUNCATED]") {
		t.Errorf("expected truncated sample, got length %d", len(got.PromptSample))
	}
	if len(got.PromptSample) > 220 {
		t.Errorf("expected sample to be capped near 200 chars, got %d", len(got.PromptSample))
	}
}

func TestLogBlocked_DisabledIsNoOp(t *testing.T) {
	path := t.TempDir() + "/events.jsonl"
	l := New(Config{Enabled: false, LocalLogPath: path})
	defer l.Close()

	l.LogBlocked(AuditEvent{UserID: "someone"})

	if _, err := os.Stat(path); err == nil {
		t.Error("expected no file to be created when telemetry is disabled")
	}
}

func TestLogBlocked_ForwardsToSidecar(t *testing.T) {
	var mu sync.Mutex
	var received AuditEvent
	gotRequest := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		json.NewDecoder(r.Body).Decode(&received)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		gotRequest <- struct{}{}
	}))
	defer srv.Close()

	l := New(Config{Enabled: true, SidecarURL: srv.URL})
	defer l.Close()

	l.LogBlocked(AuditEvent{UserID: "guest-user", Source: "outputguard", Reason: "OUTPUT_THREAT_LANGUAGE"})

	select {
	case <-gotRequest:
	case <-time.After(2 * time.Second):
		t.Fatal("sidecar never received the forwarded event")
	}

	mu.Lock()
	defer mu.Unlock()
	if received.UserID != "guest-user" || received.Source != "outputguard" {
		t.Errorf("unexpected forwarded event: %+v", received)
	}
}

func TestLogBlocked_SidecarDownDoesNotBlock(t *testing.T) {
	l := New(Config{Enabled: true, SidecarURL: "http://127.0.0.1:1", Timeout: 200 * time.Millisecond})
	defer l.Close()

	done := make(chan struct{})
	go func() {
		l.LogBlocked(AuditEvent{UserID: "x"})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("LogBlocked should return immediately even if the sidecar is unreachable (fire-and-forget)")
	}
}