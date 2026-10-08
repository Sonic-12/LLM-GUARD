package outputguard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"llmguard/proxy/internal/middleware"
)

func chatResponseBody(content string) []byte {
	body, _ := json.Marshal(map[string]any{
		"choices": []map[string]any{
			{"message": map[string]any{"role": "assistant", "content": content}},
		},
	})
	return body
}

func newRC() *middleware.RequestContext {
	return &middleware.RequestContext{RequestID: "test", Metadata: map[string]any{}}
}

func TestHandleResponse_Disabled(t *testing.T) {
	hook := New(Config{Enabled: false})
	out, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("anything goes here"))
	if err != nil {
		t.Fatalf("expected disabled hook to pass through, got error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
}

func TestHandleResponse_BlocksThreatLanguage(t *testing.T) {
	hook := New(Config{Enabled: true})
	_, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("I will kill you if you do that."))
	assertBlocked(t, err, 403)
}

func TestHandleResponse_BlocksToxicScore(t *testing.T) {
	hook := New(Config{Enabled: true})
	_, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("You are a stupid, pathetic idiot."))
	assertBlocked(t, err, 403)
}

func TestHandleResponse_AllowsBenignReply(t *testing.T) {
	hook := New(Config{Enabled: true})
	out, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("Here is a friendly summary of your request."))
	if err != nil {
		t.Fatalf("unexpected block on benign content: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
}

func TestHandleResponse_LogsHallucinationSignalWithoutBlocking(t *testing.T) {
	logPath := t.TempDir() + "/flags.jsonl"
	hook := New(Config{Enabled: true, FlagLogPath: logPath})
	defer hook.Close()
	out, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("According to a 2024 study, see https://example.com/paper for details."))
	if err != nil {
		t.Fatalf("hallucination signals must not block, got: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
}

func TestHandleResponse_LeakDetectedBlocks(t *testing.T) {
	dlp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(redactResponse{
			RedactedText: "my key is [API_KEY_0001]",
			Mapping:      map[string]string{"[API_KEY_0001]": "sk-fakekeyfortest12345"},
		})
	}))
	defer dlp.Close()

	hook := New(Config{Enabled: true, DLPBaseURL: dlp.URL})
	_, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("my key is sk-fakekeyfortest12345"))
	assertBlocked(t, err, 403)
}

func TestHandleResponse_LeakScanDownFailsClosed(t *testing.T) {
	dlp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dlp.Close()

	hook := New(Config{Enabled: true, DLPBaseURL: dlp.URL})
	_, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("a totally normal reply"))
	assertBlocked(t, err, 503)
}

func TestHandleResponse_NoLeakPassesThrough(t *testing.T) {
	dlp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(redactResponse{RedactedText: "a totally normal reply", Mapping: map[string]string{}})
	}))
	defer dlp.Close()

	hook := New(Config{Enabled: true, DLPBaseURL: dlp.URL})
	out, err := hook.HandleResponse(context.Background(), newRC(), chatResponseBody("a totally normal reply"))
	if err != nil {
		t.Fatalf("unexpected block: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected body to pass through unchanged")
	}
}

func assertBlocked(t *testing.T, err error, want int) {
	t.Helper()
	if err == nil {
		t.Fatal("expected response to be blocked, got nil error")
	}
	blocked, ok := err.(*BlockedError)
	if !ok {
		t.Fatalf("expected *BlockedError, got %T: %v", err, err)
	}
	if blocked.StatusCode != want {
		t.Errorf("expected status %d, got %d", want, blocked.StatusCode)
	}
}
