package rules

import "regexp"

var blockedPatterns = []*regexp.Regexp{
	// original signatures
	regexp.MustCompile(`\b(dan|do anything now)\b`),
	regexp.MustCompile(`ignore\s+(all\s+)?(previous|prior|system)\s+instructions`),
	regexp.MustCompile(`\b(unrestricted|developer|jailbreak(ed)?)\s+mode\b`),
	regexp.MustCompile(`disregard\s+(all\s+)?safety\s+(guidelines|protocols|rules)`),
	regexp.MustCompile(`you\s+are\s+now\s+(an\s+unfiltered|an\s+unrestricted|evil)`),

	// instruction erasure
	regexp.MustCompile(`forget\s+(everything|all|your)[\w\s']{0,25}\b(instructions|rules|guidelines|ethical\s+constraints)\b`),

	// restriction bypass
	regexp.MustCompile(`bypass\s+(restrictions|safety\s+filters|content\s+filters)`),
	regexp.MustCompile(`\badmin\s+override\b`),
	regexp.MustCompile(`\broot\s+access\b`),

	// system prompt extraction
	regexp.MustCompile(`(repeat|print|show|reveal|output|tell\s+me)\s+(your|the)\s+(system\s+prompt|(underlying\s+)?instructions)`),

	// raw override tokens
	regexp.MustCompile(`<\|system\|>`),
	regexp.MustCompile(`\[inst\]`),
	regexp.MustCompile(`###\s*instruction`),
}

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
