package dlp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"llmguard/proxy/internal/middleware"
)

// mockDLPServer spins up a fake in-memory DLP service so these tests run
// without needing real Presidio or Ollama. handlers can be overridden per
// test to simulate specific behavior (e.g. failures).
func mockDLPServer(t *testing.T, redactHandler, unmaskHandler http.HandlerFunc) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	if redactHandler != nil {
		mux.HandleFunc("/redact", redactHandler)
	}
	if unmaskHandler != nil {
		mux.HandleFunc("/unmask", unmaskHandler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func defaultRedactHandler(w http.ResponseWriter, r *http.Request) {
	var req redactRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	resp := redactResponse{
		RedactedText: "Contact at [EMAIL_ADDRESS_test1234]",
		Mapping:      map[string]string{"[EMAIL_ADDRESS_test1234]": "jane.doe@example.com"},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func defaultUnmaskHandler(w http.ResponseWriter, r *http.Request) {
	var req unmaskRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	text := req.Text
	for tok, val := range req.Mapping {
		text = strings.ReplaceAll(text, tok, val)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(unmaskResponse{Text: text})
}

func chatRequestBody(userContent string) []byte {
	body, _ := json.Marshal(map[string]any{
		"model": "llama3.2:3b",
		"messages": []map[string]any{
			{"role": "user", "content": userContent},
		},
	})
	return body
}

func chatResponseBody(assistantContent string) []byte {
	body, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-test",
		"choices": []map[string]any{
			{"message": map[string]any{"role": "assistant", "content": assistantContent}},
		},
	})
	return body
}

func TestHandleRequest_RedactsAndStoresMapping(t *testing.T) {
	srv := mockDLPServer(t, defaultRedactHandler, nil)
	hook := New(Config{BaseURL: srv.URL, Enabled: true})
	rc := &middleware.RequestContext{RequestID: "test-1", Metadata: map[string]any{}}

	out, err := hook.HandleRequest(context.Background(), rc, chatRequestBody("My email is jane.doe@example.com"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(out, &parsed)
	messages := parsed["messages"].([]any)
	content := messages[0].(map[string]any)["content"].(string)

	if content != "Contact at [EMAIL_ADDRESS_test1234]" {
		t.Errorf("expected redacted content, got: %s", content)
	}

	mapping, ok := rc.Metadata[metadataKey].(map[string]string)
	if !ok || mapping["[EMAIL_ADDRESS_test1234]"] != "jane.doe@example.com" {
		t.Errorf("expected mapping stored in rc.Metadata, got: %v", rc.Metadata[metadataKey])
	}
}

func TestHandleResponse_UnmasksUsingStoredMapping(t *testing.T) {
	srv := mockDLPServer(t, nil, defaultUnmaskHandler)
	hook := New(Config{BaseURL: srv.URL, Enabled: true})
	rc := &middleware.RequestContext{
		RequestID: "test-2",
		Metadata: map[string]any{
			metadataKey: map[string]string{"[EMAIL_ADDRESS_test1234]": "jane.doe@example.com"},
		},
	}

	out, err := hook.HandleResponse(context.Background(), rc, chatResponseBody("Contact at [EMAIL_ADDRESS_test1234]"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(out, &parsed)
	choices := parsed["choices"].([]any)
	content := choices[0].(map[string]any)["message"].(map[string]any)["content"].(string)

	if content != "Contact at jane.doe@example.com" {
		t.Errorf("expected unmasked content, got: %s", content)
	}
}

func TestHandleRequest_FailsClosedWhenRedactUnreachable(t *testing.T) {
	hook := New(Config{BaseURL: "http://127.0.0.1:1", Enabled: true}) // nothing listens here
	rc := &middleware.RequestContext{RequestID: "test-3", Metadata: map[string]any{}}

	_, err := hook.HandleRequest(context.Background(), rc, chatRequestBody("My email is jane.doe@example.com"))
	if err == nil {
		t.Fatal("expected request to be blocked when DLP service is unreachable, got no error")
	}
}

func TestHandleResponse_FailsOpenWhenUnmaskUnreachable(t *testing.T) {
	hook := New(Config{BaseURL: "http://127.0.0.1:1", Enabled: true}) // nothing listens here
	rc := &middleware.RequestContext{
		RequestID: "test-4",
		Metadata: map[string]any{
			metadataKey: map[string]string{"[EMAIL_ADDRESS_test1234]": "jane.doe@example.com"},
		},
	}

	out, err := hook.HandleResponse(context.Background(), rc, chatResponseBody("Contact at [EMAIL_ADDRESS_test1234]"))
	if err != nil {
		t.Fatalf("expected response to still return when DLP is unreachable (fail-open), got error: %v", err)
	}

	var parsed map[string]any
	json.Unmarshal(out, &parsed)
	choices := parsed["choices"].([]any)
	content := choices[0].(map[string]any)["message"].(map[string]any)["content"].(string)

	if content != "Contact at [EMAIL_ADDRESS_test1234]" {
		t.Errorf("expected token left un-restored (fail-open), got: %s", content)
	}
}

func TestHooks_NoOpWhenDisabled(t *testing.T) {
	hook := New(Config{BaseURL: "http://127.0.0.1:1", Enabled: false}) // would fail if actually called
	rc := &middleware.RequestContext{RequestID: "test-5", Metadata: map[string]any{}}

	reqBody := chatRequestBody("My email is jane.doe@example.com")
	out, err := hook.HandleRequest(context.Background(), rc, reqBody)
	if err != nil || string(out) != string(reqBody) {
		t.Errorf("expected untouched passthrough when disabled, got err=%v out=%s", err, out)
	}

	respBody := chatResponseBody("Contact at [EMAIL_ADDRESS_test1234]")
	out, err = hook.HandleResponse(context.Background(), rc, respBody)
	if err != nil || string(out) != string(respBody) {
		t.Errorf("expected untouched passthrough when disabled, got err=%v out=%s", err, out)
	}
}
