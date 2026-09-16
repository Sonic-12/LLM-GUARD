package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"llmguard/proxy/internal/middleware"
	"llmguard/proxy/internal/threatdetect"
)

type Config struct {
	Enabled         bool
	MaxChars        int
	FirewallURL     string
	DecisionLogPath string // empty disables logging
}

type BlockedError struct {
	StatusCode int
	Body       []byte
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("blocked by rules engine: status=%d", e.StatusCode)
}

// decisionLogEntry is one line written to DecisionLogPath per request
// evaluated by the rules hook - the persistent record needed for audit
// and for a future human-review dashboard (Week 4).
type decisionLogEntry struct {
	Timestamp      string  `json:"timestamp"`
	RequestID      string  `json:"request_id"`
	Allowed        bool    `json:"allowed"`
	Reason         string  `json:"reason,omitempty"`
	JailbreakScore float64 `json:"jailbreak_score,omitempty"`
	ReviewRequired bool    `json:"review_required,omitempty"`
}

type Hook struct {
	cfg     Config
	client  *threatdetect.FirewallClient
	logFile *os.File
	logMu   sync.Mutex
}

func New(cfg Config) *Hook {
	var c *threatdetect.FirewallClient
	if cfg.FirewallURL != "" {
		c = threatdetect.NewFirewallClient(cfg.FirewallURL)
	}

	h := &Hook{cfg: cfg, client: c}

	if cfg.DecisionLogPath != "" {
		f, err := os.OpenFile(cfg.DecisionLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Logging failure must not block the request pipeline - surface
			// it explicitly instead of silently swallowing (per project rules).
			fmt.Fprintf(os.Stderr, "rules: could not open decision log %q: %v (decision logging disabled)\n", cfg.DecisionLogPath, err)
		} else {
			h.logFile = f
		}
	}

	return h
}

func (h *Hook) Name() string { return "rules" }

func (h *Hook) logDecision(rc *middleware.RequestContext, decision PipelineDecision) {
	if h.logFile == nil {
		return
	}
	entry := decisionLogEntry{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		RequestID:      rc.RequestID,
		Allowed:        decision.Allowed,
		Reason:         decision.Reason,
		JailbreakScore: decision.JailbreakScore,
		ReviewRequired: decision.ReviewRequired,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rules: could not marshal decision log entry: %v\n", err)
		return
	}

	h.logMu.Lock()
	defer h.logMu.Unlock()
	if _, err := h.logFile.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "rules: could not write decision log entry: %v\n", err)
	}
}

func (h *Hook) HandleRequest(ctx context.Context, rc *middleware.RequestContext, body []byte) ([]byte, error) {
	if !h.cfg.Enabled {
		return body, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return body, nil
	}
	rawMessages, _ := parsed["messages"].([]any)

	for _, raw := range rawMessages {
		msgMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)
		if role != "user" {
			continue
		}
		content, _ := msgMap["content"].(string)

		decision := EvaluatePrompt(content, h.cfg.MaxChars, h.client)
		h.logDecision(rc, decision)

		if !decision.Allowed {
			payload, err := json.Marshal(decision)
			if err != nil {
				payload = []byte(fmt.Sprintf(`{"allowed":false,"reason":%q}`, decision.Reason))
			}
			return nil, &BlockedError{StatusCode: decision.StatusCode, Body: payload}
		}
	}

	return body, nil
}
