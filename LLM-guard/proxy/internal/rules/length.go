package rules

import (
	"strings"
	"unicode/utf8"
)

const DefaultMaxChars = 4000

// CheckLength validates that the prompt is not empty and within rune limits.
func CheckLength(prompt string, maxChars int) (bool, *RuleViolation) {
	trimmed := strings.TrimSpace(prompt)
	if len(trimmed) == 0 {
		return false, &RuleViolation{
			Rule:    "EMPTY_PROMPT",
			Message: "Prompt cannot be empty or contain only whitespace",
		}
	}

	limit := maxChars
	if limit <= 0 {
		limit = DefaultMaxChars
	}

	runeCount := utf8.RuneCountInString(prompt)
	if runeCount > limit {
		return false, &RuleViolation{
			Rule:    "MAX_LENGTH_EXCEEDED",
			Message: "Prompt exceeds maximum allowed character boundary",
		}
	}

	return true, nil
}
