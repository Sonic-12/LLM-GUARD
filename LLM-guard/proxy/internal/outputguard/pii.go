package outputguard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type redactRequest struct {
	Text string `json:"text"`
}

type redactResponse struct {
	RedactedText string            `json:"redacted_text"`
	Mapping      map[string]string `json:"mapping"`
}

func (h *Hook) checkLeak(ctx context.Context, text string) (bool, error) {
	payload, err := json.Marshal(redactRequest{Text: text})
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.DLPBaseURL+"/redact", bytes.NewReader(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("dlp /redact returned status %d", resp.StatusCode)
	}

	var rr redactResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return false, err
	}
	return len(rr.Mapping) > 0, nil
}