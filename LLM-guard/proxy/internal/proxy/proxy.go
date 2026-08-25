package proxy

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"llmguard/proxy/internal/config"
	"llmguard/proxy/internal/middleware"
)

type Server struct {
	cfg   *config.Config
	chain *middleware.Chain
	http  *http.Client
}

func New(cfg *config.Config, chain *middleware.Chain) *Server {
	return &Server{
		cfg:   cfg,
		chain: chain,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rc := &middleware.RequestContext{
		RequestID: newRequestID(),
		Metadata:  map[string]any{},
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	body, err = s.chain.RunPre(r.Context(), rc, body)
	if err != nil {
		log.Printf("[%s] blocked at pre-request: %v", rc.RequestID, err)
		http.Error(w, fmt.Sprintf("request blocked: %v", err), http.StatusForbidden)
		return
	}

	respBody, status, err := s.forward(r, body)
	if err != nil {
		log.Printf("[%s] upstream error: %v", rc.RequestID, err)
		http.Error(w, "upstream LLM request failed", http.StatusBadGateway)
		return
	}

	respBody, err = s.chain.RunPost(r.Context(), rc, respBody)
	if err != nil {
		log.Printf("[%s] blocked at post-response: %v", rc.RequestID, err)
		http.Error(w, fmt.Sprintf("response blocked: %v", err), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-LLMGuard-Request-ID", rc.RequestID)
	w.WriteHeader(status)
	w.Write(respBody)

	log.Printf("[%s] %s %s -> %d (%s)", rc.RequestID, r.Method, r.URL.Path, status, time.Since(start))
}

func (s *Server) forward(r *http.Request, body []byte) ([]byte, int, error) {
	target := s.cfg.Upstream.BaseURL + r.URL.Path

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	if s.cfg.Upstream.APIKeyEnv != "" {
		if key := os.Getenv(s.cfg.Upstream.APIKeyEnv); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	return respBody, resp.StatusCode, nil
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}