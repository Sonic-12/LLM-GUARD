package outputguard

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"llmguard/proxy/internal/middleware"
)

type Config struct {
	Enabled     bool
	DLPBaseURL  string
	FlagLogPath string
	Timeout     time.Duration
}

type BlockedError struct {
	StatusCode int
	Body       []byte
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("blocked by output guard: status=%d", e.StatusCode)
}

func blocked(status int, reason string) error {
	payload, _ := json.Marshal(map[string]any{"allowed": false, "reason": reason})
	return &BlockedError{StatusCode: status, Body: payload}
}

type flagLogEntry struct {
	Timestamp string   `json:"timestamp"`
	RequestID string   `json:"request_id"`
	Flags     []string `json:"flags"`
}

type Hook struct {
	cfg     Config
	client  *http.Client
	logFile *os.File
	logMu   sync.Mutex
}

func New(cfg Config) *Hook {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	h := &Hook{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}

	if cfg.FlagLogPath != "" {
		f, err := os.OpenFile(cfg.FlagLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "outputguard: could not open flag log %q: %v (flag logging disabled)\n", cfg.FlagLogPath, err)
		} else {
			h.logFile = f
		}
	}
	return h
}

func (h *Hook) Name() string { return "outputguard" }
func (h *Hook) Close() error {
	if h.logFile == nil {
		return nil
	}
	return h.logFile.Close()
}

func (h *Hook) logFlags(rc *middleware.RequestContext, flags []string) {
	if h.logFile == nil || len(flags) == 0 {
		return
	}
	entry := flagLogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		RequestID: rc.RequestID,
		Flags:     flags,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "outputguard: could not marshal flag log entry: %v\n", err)
		return
	}

	h.logMu.Lock()
	defer h.logMu.Unlock()
	if _, err := h.logFile.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "outputguard: could not write flag log entry: %v\n", err)
	}
}

func (h *Hook) HandleResponse(ctx context.Context, rc *middleware.RequestContext, body []byte) ([]byte, error) {
	if !h.cfg.Enabled {
		return body, nil
	}

	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return body, nil
	}
	choices, _ := parsed["choices"].([]any)

	for _, c := range choices {
		choiceMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		msgMap, ok := choiceMap["message"].(map[string]any)
		if !ok {
			continue
		}
		content, ok := msgMap["content"].(string)
		if !ok || content == "" {
			continue
		}

		if hit := checkToxicity(content); hit != nil {
			log.Printf("[%s] outputguard: blocked toxic output: %s", rc.RequestID, hit.Rule)
			return nil, blocked(403, hit.Rule)
		}

		if h.cfg.DLPBaseURL != "" {
			leaked, err := h.checkLeak(ctx, content)
			if err != nil {
				log.Printf("[%s] outputguard: leak scan failed, blocking response: %v", rc.RequestID, err)
				return nil, blocked(503, "OUTPUT_SCAN_SERVICE_UNAVAILABLE_FAIL_CLOSED")
			}
			if leaked {
				log.Printf("[%s] outputguard: blocked leaked sensitive entity in output", rc.RequestID)
				return nil, blocked(403, "OUTPUT_PII_LEAK")
			}
		}

		if flags := checkHallucinationSignals(content); len(flags) > 0 {
			log.Printf("[%s] outputguard: flagged (not blocked): %v", rc.RequestID, flags)
			h.logFlags(rc, flags)
		}
	}

	return body, nil
}
