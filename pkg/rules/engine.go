package rules

import (
	"fmt"

	"github.com/Sonic-12/LLM-GUARD/pkg/client"
)

type PipelineDecision struct {
	Allowed        bool    `json:"allowed"`
	StatusCode     int     `json:"status_code"`
	Reason         string  `json:"reason,omitempty"`
	JailbreakScore float64 `json:"jailbreak_score,omitempty"`
}

// EvaluatePrompt runs the short-circuit multi-tier defense pipeline:
// Tier 1: Length bounds (<0.1ms)
// Tier 2: Signature Regex blocklist (<0.2ms)
// Tier 3: Python Semantic ML Classifier (~5-15ms)
func EvaluatePrompt(prompt string, maxChars int, client *client.FirewallClient) PipelineDecision {
	// 1. Length bounds evaluation
	if valid, violation := CheckLength(prompt, maxChars); !valid {
		return PipelineDecision{
			Allowed:    false,
			StatusCode: 400,
			Reason:     violation.Rule,
		}
	}

	// 2. Keyword & signature blocklist evaluation
	if valid, violation := CheckBlocklist(prompt); !valid {
		return PipelineDecision{
			Allowed:    false,
			StatusCode: 403,
			Reason:     fmt.Sprintf("%s: %s", violation.Rule, violation.MatchedPattern),
		}
	}

	// 3. Python ML Semantic Jailbreak check (if client provided)
	if client != nil {
		resp, err := client.InspectPrompt(prompt, 0.50)
		if err != nil {
			// Fail-safe or fallback on connection failure
			return PipelineDecision{
				Allowed:    true,
				StatusCode: 200,
				Reason:     "INSPECTION_BYPASSED_SERVICE_UNAVAILABLE",
			}
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
			}
		}

		return PipelineDecision{
			Allowed:        true,
			StatusCode:     200,
			JailbreakScore: resp.JailbreakScore,
		}
	}

	return PipelineDecision{Allowed: true, StatusCode: 200}
}
