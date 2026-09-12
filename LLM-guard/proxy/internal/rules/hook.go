package rules

import (
	"context"
	"encoding/json"
	"fmt"

	"llmguard/proxy/internal/middleware"
	"llmguard/proxy/internal/threatdetect"
)

type Config struct {
	Enabled     bool
	MaxChars    int
	FirewallURL string
}

// BlockedError carries the pipeline decision so proxy.go can respond with
// the correct status code (400 length / 403 blocklist,ML) and a JSON body,
// instead of the flat 403 text used for generic hook errors.
type BlockedError struct {
	StatusCode int
	Body       []byte
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("blocked by rules engine: status=%d", e.StatusCode)
}

type Hook struct {
	cfg    Config
	client *threatdetect.FirewallClient
}

func New(cfg Config) *Hook {
	var c *threatdetect.FirewallClient
	if cfg.FirewallURL != "" {
		c = threatdetect.NewFirewallClient(cfg.FirewallURL)
	}
	return &Hook{cfg: cfg, client: c}
}

func (h *Hook) Name() string { return "rules" }

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
