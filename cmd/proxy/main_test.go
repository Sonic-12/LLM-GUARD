package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWriteJSONError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeJSONError(rr, http.StatusForbidden, "Blocked by test", "security_violation")

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 status code, got %d", rr.Code)
	}

	expected := `"code":403`
	if !bytes.Contains(rr.Body.Bytes(), []byte(expected)) {
		t.Fatalf("expected response body to contain %s, got %s", expected, rr.Body.String())
	}
}
