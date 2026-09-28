package telemetry

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type AuditEvent struct {
	Timestamp      string  `json:"timestamp"`
	EventID        string  `json:"event_id"`
	UserID         string  `json:"user_id"`
	ClientIP       string  `json:"client_ip"`
	PromptLength   int     `json:"prompt_length"`
	Allowed        bool    `json:"allowed"`
	StatusCode     int     `json:"status_code"`
	Reason         string  `json:"reason,omitempty"`
	RuleTriggered  string  `json:"rule_triggered,omitempty"`
	JailbreakScore float64 `json:"jailbreak_score"`
	PromptSample   string  `json:"prompt_sample"`
}

type SIEMLogger struct {
	mu     sync.Mutex
	writer io.Writer
}

func NewSIEMLogger(writer io.Writer) *SIEMLogger {
	if writer == nil {
		writer = os.Stdout
	}
	return &SIEMLogger{writer: writer}
}

func (s *SIEMLogger) LogEvent(event AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Sanitize prompt sample length to prevent excessive log bloat
	if len(event.PromptSample) > 200 {
		event.PromptSample = event.PromptSample[:200] + "...[TRUNCATED]"
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}

	_, err = s.writer.Write(append(bytes, '\n'))
	return err
}
