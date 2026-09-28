package rules

// RuleViolation encapsulates details regarding a failed heuristic check.
type RuleViolation struct {
	Rule           string `json:"rule"`
	Message        string `json:"message"`
	MatchedPattern string `json:"matched_pattern,omitempty"`
}

// CheckHeuristics runs deterministic checks sequentially:
// 1. Prompt bounds (length/empty checks)
// 2. Signature and keyword regex blocklists
func CheckHeuristics(prompt string, maxChars int) (bool, *RuleViolation) {
	if valid, violation := CheckLength(prompt, maxChars); !valid {
		return false, violation
	}

	if valid, violation := CheckBlocklist(prompt); !valid {
		return false, violation
	}

	return true, nil
}
