package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"llmguard/proxy/internal/config"
	"llmguard/proxy/internal/middleware"
	"llmguard/proxy/internal/outputguard"
	"llmguard/proxy/internal/rbac"
	"llmguard/proxy/internal/rules"
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
		http:  &http.Client{Timeout: 90 * time.Second},
	}
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rc := &middleware.RequestContext{
		RequestID: newRequestID(),
		Metadata:  map[string]any{},
	}

	rc.Metadata[rbac.MetadataAuthHeader] = r.Header.Get("Authorization")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	preStart := time.Now()
	body, err = s.chain.RunPre(r.Context(), rc, body)
	preDuration := time.Since(preStart)
	if err != nil {
		log.Printf("[%s] blocked at pre-request: %v", rc.RequestID, err)
		var rulesBlocked *rules.BlockedError
		if errors.As(err, &rulesBlocked) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(rulesBlocked.StatusCode)
			w.Write(rulesBlocked.Body)
			return
		}
		var rbacBlocked *rbac.BlockedError
		if errors.As(err, &rbacBlocked) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(rbacBlocked.StatusCode)
			w.Write(rbacBlocked.Body)
			return
		}
		http.Error(w, fmt.Sprintf("request blocked: %v", err), http.StatusForbidden)
		return
	}

	respBody, status, err := s.forward(r, rc.RequestID, body)
	if err != nil {
		log.Printf("[%s] upstream error: %v", rc.RequestID, err)
		http.Error(w, "upstream LLM request failed", http.StatusBadGateway)
		return
	}

	postStart := time.Now()
	respBody, err = s.chain.RunPost(r.Context(), rc, respBody)
	postDuration := time.Since(postStart)
	if err != nil {
		log.Printf("[%s] blocked at post-response: %v", rc.RequestID, err)
		var outputBlocked *outputguard.BlockedError
		if errors.As(err, &outputBlocked) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(outputBlocked.StatusCode)
			w.Write(outputBlocked.Body)
			return
		}
		http.Error(w, fmt.Sprintf("response blocked: %v", err), http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-LLMGuard-Request-ID", rc.RequestID)
	w.WriteHeader(status)
	w.Write(respBody)

	log.Printf("[%s] %s %s -> %d total=%s pre_hooks=%s post_hooks=%s", rc.RequestID, r.Method, r.URL.Path, status, time.Since(start), preDuration, postDuration)
}

func (s *Server) forward(r *http.Request, requestID string, body []byte) ([]byte, int, error) {
	target := s.cfg.Upstream.BaseURL + r.URL.Path

	requestedModel, err := extractModel(body)
	if err != nil {
		return nil, 0, fmt.Errorf("could not parse request body: %w", err)
	}

	if s.cfg.Upstream.PremiumModel == "" || requestedModel != s.cfg.Upstream.PremiumModel {
		defaultBody, err := withModel(body, s.cfg.Upstream.DefaultModel, s.cfg.Upstream.DefaultNumPredict)
		if err != nil {
			return nil, 0, err
		}
		return s.doForward(r.Context(), r.Method, target, defaultBody)
	}

	premiumBody, err := withModel(body, s.cfg.Upstream.PremiumModel, s.cfg.Upstream.PremiumNumPredict)
	if err != nil {
		return nil, 0, err
	}

	premiumCtx, cancel := context.WithTimeout(r.Context(), s.cfg.Upstream.PremiumTimeout)
	defer cancel()

	respBody, status, err := s.doForward(premiumCtx, r.Method, target, premiumBody)
	if err == nil {
		return respBody, status, nil
	}
	if !errors.Is(premiumCtx.Err(), context.DeadlineExceeded) {
		return nil, 0, err
	}

	log.Printf("[%s] premium model timed out after %s, falling back to %s", requestID, s.cfg.Upstream.PremiumTimeout, s.cfg.Upstream.DefaultModel)

	fallbackBody, ferr := withModel(body, s.cfg.Upstream.DefaultModel, s.cfg.Upstream.FallbackNumPredict)
	if ferr != nil {
		return nil, 0, ferr
	}

	respBody, status, err = s.doForward(r.Context(), r.Method, target, fallbackBody)
	if err != nil {
		return nil, 0, err
	}
	log.Printf("[%s] fallback model %s answered", requestID, s.cfg.Upstream.DefaultModel)
	return respBody, status, nil
}

func (s *Server) doForward(ctx context.Context, method, target string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
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

func extractModel(body []byte) (string, error) {
	var m struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &m); err != nil {
		return "", err
	}
	return m.Model, nil
}

func withModel(body []byte, model string, maxTokens int) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("could not parse request to apply model: %w", err)
	}
	m["model"] = model
	if maxTokens > 0 {
		m["max_tokens"] = maxTokens
	}
	return json.Marshal(m)
}

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
