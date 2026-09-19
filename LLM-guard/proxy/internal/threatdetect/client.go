package threatdetect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type InspectionRequest struct {
	Prompt string `json:"prompt"`
}

type InspectionResponse struct {
	Allowed        bool    `json:"allowed"`
	JailbreakScore float64 `json:"jailbreak_score"`
	ReviewRequired bool    `json:"review_required"`
	Reason         *string `json:"reason"`
}

type FirewallClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewFirewallClient(baseURL string) *FirewallClient {
	return &FirewallClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 3000 * time.Millisecond,
		},
	}
}

func (c *FirewallClient) InspectPrompt(prompt string) (*InspectionResponse, error) {
	reqBody, err := json.Marshal(InspectionRequest{Prompt: prompt})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize inspection request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/firewall/inspect", c.baseURL)
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("firewall service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firewall returned non-200 status: %d", resp.StatusCode)
	}

	var inspectionResp InspectionResponse
	if err := json.NewDecoder(resp.Body).Decode(&inspectionResp); err != nil {
		return nil, fmt.Errorf("failed to decode firewall response: %w", err)
	}

	return &inspectionResp, nil
}
