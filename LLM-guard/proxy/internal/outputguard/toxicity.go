package outputguard

import (
	"fmt"
	"regexp"
	"strings"

	"llmguard/proxy/internal/rules"
)

type toxicityHit struct {
	Rule string
}

var threatPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\bi(\s*'?ll| will)?\s*(kill|hurt|attack|beat)\s+you\b`),
	regexp.MustCompile(`\byou\s+(will|gonna|are going to)\s+(die|suffer|regret)\b`),
	regexp.MustCompile(`\bgo\s+kill\s+yourself\b`),
}

var insultWords = []string{
	"idiot", "moron", "stupid", "worthless", "pathetic", "loser", "scum", "trash",
}

const toxicityScoreThreshold = 2

func checkToxicity(text string) *toxicityHit {
	normalized := rules.NormalizeForDetection(text)

	for _, p := range threatPatterns {
		if p.MatchString(normalized) {
			return &toxicityHit{Rule: "OUTPUT_THREAT_LANGUAGE"}
		}
	}

	score := 0
	for _, w := range insultWords {
		if strings.Contains(normalized, w) {
			score++
		}
	}
	if score >= toxicityScoreThreshold {
		return &toxicityHit{Rule: fmt.Sprintf("OUTPUT_TOXIC_LANGUAGE_SCORE_%d", score)}
	}
	return nil
}
