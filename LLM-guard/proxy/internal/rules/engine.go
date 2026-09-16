package rules

import (
	"fmt"

	"llmguard/proxy/internal/threatdetect"
)

type PipelineDecision struct {
	Allowed        bool    `json:"allowed"`
	StatusCode     int     `json:"status_code"`
	Reason         string  `json:"reason,omitempty"`
	JailbreakScore float64 `json:"jailbreak_score,omitempty"`
	ReviewRequired bool    `json:"review_required,omitempty"`
}

// EvaluatePrompt runs the short-circuit multi-tier defense pipeline:
// Tier 1: Length bounds (<0.1ms)
// Tier 2: Signature Regex blocklist (<0.2ms)
// Tier 3: Python Semantic ML Classifier (~5-15ms), 3-band confidence
func EvaluatePrompt(prompt string, maxChars int, client *threatdetect.FirewallClient) PipelineDecision {
	if valid, violation := CheckLength(prompt, maxChars); !valid {
		return PipelineDecision{Allowed: false, StatusCode: 400, Reason: violation.Rule}
	}

	if valid, violation := CheckBlocklist(prompt); !valid {
		return PipelineDecision{
			Allowed:    false,
			StatusCode: 403,
			Reason:     fmt.Sprintf("%s: %s", violation.Rule, violation.MatchedPattern),
		}
	}

	if client != nil {
		resp, err := client.InspectPrompt(prompt)
		if err != nil {
			// Fail-open on connection failure
			return PipelineDecision{Allowed: true, StatusCode: 200, Reason: "INSPECTION_BYPASSED_SERVICE_UNAVAILABLE"}
		}

		if !resp.Allowed {
			reason := "SEMANTIC_JAILBREAK_DETECTED"
			if resp.Reason != nil {
				reason = *resp.Reason
			}
			return PipelineDecision{
				Allowed:        false,
				StatusCode:     403,
				Reason:         reason,
				JailbreakScore: resp.JailbreakScore,
				ReviewRequired: resp.ReviewRequired,
			}
		}

		return PipelineDecision{Allowed: true, StatusCode: 200, JailbreakScore: resp.JailbreakScore}
	}

	return PipelineDecision{Allowed: true, StatusCode: 200}
}