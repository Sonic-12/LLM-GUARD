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
}

func EvaluatePrompt(prompt string, maxChars int, client *threatdetect.FirewallClient) PipelineDecision {
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
			// Fail-open on connection failure (matches DLP's fail-open policy on /unmask)
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
