package rules

import "strings"


var homoglyphs = map[rune]rune{
	'а': 'a', 'А': 'A',
	'е': 'e', 'Е': 'E',
	'о': 'o', 'О': 'O',
	'р': 'p', 'Р': 'P',
	'с': 'c', 'С': 'C',
	'у': 'y', 'У': 'Y',
	'х': 'x', 'Х': 'X',
	'і': 'i', 'І': 'I',
	'ѕ': 's', 'ј': 'j', 'ԁ': 'd', 'ⅼ': 'l',
	'Ι': 'I', 'ο': 'o', 'Ο': 'O',
	'Α': 'A', 'Β': 'B', 'Ε': 'E', 'Ζ': 'Z', 'Η': 'H', 'Κ': 'K', 'Μ': 'M', 'Ν': 'N', 'Τ': 'T', 'Υ': 'Y', 'Χ': 'X',
}

var zeroWidth = map[rune]bool{
	'\u200b': true, '\u200c': true, '\u200d': true, '\ufeff': true, '\u2060': true,
}

func NormalizeForDetection(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if zeroWidth[r] {
			continue
		}
		if r >= 0xFF01 && r <= 0xFF5E {
			r = r - 0xFEE0 // fullwidth -> ASCII
		}
		if repl, ok := homoglyphs[r]; ok {
			r = repl
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}