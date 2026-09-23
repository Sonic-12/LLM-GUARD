package outputguard

import "regexp"

var urlPattern = regexp.MustCompile(`https?://[^\s)]+`)
var citationPattern = regexp.MustCompile(`(?i)\b(according to|studies show|research (indicates|shows)|a \d{4} study|cited in)\b`)

func checkHallucinationSignals(text string) []string {
	var flags []string
	if urlPattern.MatchString(text) {
		flags = append(flags, "UNVERIFIABLE_URL")
	}
	if citationPattern.MatchString(text) {
		flags = append(flags, "UNVERIFIABLE_CITATION_CLAIM")
	}
	return flags
}