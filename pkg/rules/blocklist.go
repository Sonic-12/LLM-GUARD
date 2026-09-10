package rules

import (
	"regexp"
)

var blockedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(DAN|do anything now)\b`),
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|system)\s+instructions`),
	regexp.MustCompile(`(?i)\b(unrestricted|developer|jailbreak(ed)?)\s+mode\b`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?safety\s+(guidelines|protocols|rules)`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+(an\s+unfiltered|an\s+unrestricted|evil)`),
}

// CheckBlocklist scans the prompt against precompiled adversarial signatures.
func CheckBlocklist(prompt string) (bool, *RuleViolation) {
	for _, pattern := range blockedPatterns {
		if pattern.MatchString(prompt) {
			return false, &RuleViolation{
				Rule:           "SIGNATURE_MATCH",
				Message:        "Prompt matches a known adversarial or jailbreak signature",
				MatchedPattern: pattern.String(),
			}
		}
	}
	return true, nil
}
