package dlp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"llmguard/proxy/internal/middleware"
)

type Config struct {
	BaseURL string
	Enabled bool
	Timeout time.Duration
}

const metadataKey = "dlp_mapping"

type Hook struct {
	cfg    Config
	client *http.Client
}

func New(cfg Config) *Hook {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &Hook{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

func (h *Hook) Name() string { return "dlp" }

type redactRequest struct {
	Text string `json:"text"`
}

type redactResponse struct {
	RedactedText string            `json:"redacted_text"`
	Mapping      map[string]string `json:"mapping"`
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

	fullMapping := map[string]string{}

	for _, raw := range rawMessages {
		msgMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msgMap["role"].(string)
		content, _ := msgMap["content"].(string)
		if role != "user" || content == "" {
			continue
		}

		rr, err := h.redact(ctx, content)
		if err != nil {
			log.Printf("[%s] dlp: /redact call failed, blocking request: %v", rc.RequestID, err)
			return nil, fmt.Errorf("dlp service unavailable: %w", err)
		}
		for tok, val := range rr.Mapping {
			fullMapping[tok] = val
		}
		msgMap["content"] = rr.RedactedText
	}

	if len(fullMapping) > 0 {
		rc.Metadata[metadataKey] = fullMapping
	}

	out, err := json.Marshal(parsed)
	if err != nil {
		return body, nil
	}
	return out, nil
}

type unmaskRequest struct {
	Text    string            `json:"text"`
	Mapping map[string]string `json:"mapping"`
}

type unmaskResponse struct {
	Text string `json:"text"`
}

func (h *Hook) HandleResponse(ctx context.Context, rc *middleware.RequestContext, body []byte) ([]byte, error) {
	if !h.cfg.Enabled {
		return body, nil
	}

	mappingAny, ok := rc.Metadata[metadataKey]
	if !ok {
		return body, nil
	}
	mapping, ok := mappingAny.(map[string]string)
	if !ok || len(mapping) == 0 {
		return body, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return body, nil
	}
	choices, _ := parsed["choices"].([]any)

	for _, c := range choices {
		choiceMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		msgMap, ok := choiceMap["message"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := msgMap["content"].(string)
		if !ok || content == "" {
			continue
		}

		unmasked, err := h.unmask(ctx, content, mapping)
		if err != nil {
			log.Printf("[%s] dlp: /unmask call failed, returning response with tokens intact: %v", rc.RequestID, err)
			continue
		}
		msgMap["content"] = unmasked
	}

	out, err := json.Marshal(parsed)
	if err != nil {
		return body, nil
	}
	return out, nil
}

func (h *Hook) redact(ctx context.Context, text string) (redactResponse, error) {
	payload, err := json.Marshal(redactRequest{Text: text})
	if err != nil {
		return redactResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.BaseURL+"/redact", bytes.NewReader(payload))
	if err != nil {
		return redactResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return redactResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return redactResponse{}, fmt.Errorf("dlp /redact returned status %d", resp.StatusCode)
	}

	var rr redactResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return redactResponse{}, err
	}
	return rr, nil
}

func (h *Hook) unmask(ctx context.Context, text string, mapping map[string]string) (string, error) {
	payload, err := json.Marshal(unmaskRequest{Text: text, Mapping: mapping})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.BaseURL+"/unmask", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dlp /unmask returned status %d", resp.StatusCode)
	}

	var ur unmaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&ur); err != nil {
		return "", err
	}
	return ur.Text, nil
}
