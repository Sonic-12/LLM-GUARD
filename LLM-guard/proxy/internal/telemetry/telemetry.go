package telemetry
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"
)

type AuditEvent struct {
	Timestamp      string  `json:"timestamp"`
	EventID        string  `json:"event_id"`
	UserID         string  `json:"user_id"`
	ClientIP       string  `json:"client_ip"`
	Source         string  `json:"source"` 
	PromptLength   int     `json:"prompt_length"`
	Allowed        bool    `json:"allowed"`
	StatusCode     int     `json:"status_code"`
	Reason         string  `json:"reason,omitempty"`
	RuleTriggered  string  `json:"rule_triggered,omitempty"`
	JailbreakScore float64 `json:"jailbreak_score,omitempty"`
	PromptSample   string  `json:"prompt_sample"`
}

type Config struct {
	Enabled      bool
	LocalLogPath string 
	SidecarURL   string 
	Timeout      time.Duration
}

type Logger struct {
	cfg     Config
	client  *http.Client
	logFile *os.File
	mu      sync.Mutex
}

func New(cfg Config) *Logger {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	l := &Logger{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
	if cfg.Enabled && cfg.LocalLogPath != "" {
		f, err := os.OpenFile(cfg.LocalLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "telemetry: could not open local log %q: %v (local logging disabled)\n", cfg.LocalLogPath, err)
		} else {
			l.logFile = f
		}
	}
	return l
}

func (l *Logger) Close() error {
	if l.logFile == nil {
		return nil
	}
	return l.logFile.Close()
}


func NewEventID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "evt-" + hex.EncodeToString(b)
}

func (l *Logger) LogBlocked(event AuditEvent) {
	if !l.cfg.Enabled {
		return
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}
	if event.EventID == "" {
		event.EventID = NewEventID()
	}
	if len(event.PromptSample) > 200 {
		event.PromptSample = event.PromptSample[:200] + "...[TRUNCATED]"
	}

	l.writeLocal(event)
	if l.cfg.SidecarURL != "" {
		go l.forward(event) 
	}
}

func (l *Logger) writeLocal(event AuditEvent) {
	if l.logFile == nil {
		return
	}
	line, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: marshal error: %v\n", err)
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.logFile.Write(append(line, '\n')); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: local write error: %v\n", err)
	}
}

func (l *Logger) forward(event AuditEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), l.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, l.cfg.SidecarURL+"/api/v1/telemetry/ingest", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := l.client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: sidecar forward failed: %v\n", err)
		return
	}
	defer resp.Body.Close()
}