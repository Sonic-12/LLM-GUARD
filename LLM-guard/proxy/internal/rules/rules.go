package rules

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"llmguard/proxy/internal/middleware"
)

type Config struct {
	MaxPromptLength       int
	BlockedKeywords       []string
	SystemOverridePhrases []string
}

func LoadConfig(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	cfg := Config{MaxPromptLength: 4000}
	var target *[]string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := stripComment(scanner.Text())
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "- ") {
			if target == nil {
				continue
			}
			*target = append(*target, strings.Trim(strings.TrimSpace(trimmed[1:]), `"'`))
			continue
		}

		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		val := ""
		if len(parts) == 2 {
			val = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
		}

		switch key {
		case "max_prompt_length":
			target = nil
			if n, err := strconv.Atoi(val); err == nil {
				cfg.MaxPromptLength = n
			}
		case "blocked_keywords":
			target = &cfg.BlockedKeywords
		case "system_override_phrases":
			target = &cfg.SystemOverridePhrases
		default:
			target = nil
		}
	}
	return cfg, scanner.Err()
}

func stripComment(line string) string {
	inQuote, q := false, byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if inQuote {
			if c == q {
				inQuote = false
			}
			continue
		}
		if c == '"' || c == '\'' {
			inQuote, q = true, c
			continue
		}
		if c == '#' {
			return line[:i]
		}
	}
	return line
}

var homoglyphs = map[rune]rune{
	'а': 'a', 'е': 'e', 'о': 'o', 'р': 'p', 'с': 'c', 'у': 'y', 'х': 'x',
	'і': 'i', 'ѕ': 's', 'ј': 'j', 'һ': 'h', 'ԁ': 'd',
}

var zeroWidth = map[rune]bool{
	'\u200b': true, '\u200c': true, '\u200d': true, '\ufeff': true, '\u2060': true,
}

func normalize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if zeroWidth[r] {
			continue
		}
		if r >= '\u202a' && r <= '\u202e' || r >= '\u2066' && r <= '\u2069' {
			continue
		}
		if r >= '\uff21' && r <= '\uff3a' {
			r = r - '\uff21' + 'A'
		} else if r >= '\uff41' && r <= '\uff5a' {
			r = r - '\uff41' + 'a'
		}
		if repl, ok := homoglyphs[r]; ok {
			r = repl
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func compilePattern(phrase string) *regexp.Regexp {
	words := strings.Fields(normalize(phrase))
	for i, w := range words {
		words[i] = regexp.QuoteMeta(w)
	}
	return regexp.MustCompile(`\b` + strings.Join(words, `\W+`) + `\b`)
}

var errBlocked = errors.New("request violates content policy")

type Hook struct {
	cfg      Config
	keywords []*regexp.Regexp
	override []*regexp.Regexp
}

func New(cfg Config) *Hook {
	h := &Hook{cfg: cfg}
	for _, kw := range cfg.BlockedKeywords {
		h.keywords = append(h.keywords, compilePattern(kw))
	}
	for _, ph := range cfg.SystemOverridePhrases {
		h.override = append(h.override, compilePattern(ph))
	}
	return h
}

func (h *Hook) Name() string { return "rules-engine" }

type chatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func (h *Hook) HandleRequest(ctx context.Context, rc *middleware.RequestContext, body []byte) ([]byte, error) {
	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("[%s] rules: invalid request format: %v", rc.RequestID, err)
		return nil, errBlocked
	}

	var text strings.Builder
	for _, m := range req.Messages {
		if m.Role == "user" {
			text.WriteString(m.Content)
			text.WriteString(" ")
		}
	}
	raw := text.String()

	if len(raw) > h.cfg.MaxPromptLength {
		log.Printf("[%s] rules: blocked, length %d exceeds max %d", rc.RequestID, len(raw), h.cfg.MaxPromptLength)
		return nil, errBlocked
	}

	norm := normalize(raw)

	for i, p := range h.keywords {
		if p.MatchString(norm) {
			log.Printf("[%s] rules: blocked, matched blocked_keywords[%d]=%q", rc.RequestID, i, h.cfg.BlockedKeywords[i])
			return nil, errBlocked
		}
	}
	for i, p := range h.override {
		if p.MatchString(norm) {
			log.Printf("[%s] rules: blocked, matched system_override_phrases[%d]=%q", rc.RequestID, i, h.cfg.SystemOverridePhrases[i])
			return nil, errBlocked
		}
	}

	return body, nil
}
