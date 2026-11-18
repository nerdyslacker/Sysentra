package daemon

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"sysentra/internal/security"
)

type AuditEvent struct {
	Time      time.Time          `json:"time"`
	Stage     string             `json:"stage"`
	Outcome   string             `json:"outcome"`
	Sender    string             `json:"sender,omitempty"`
	RequestID string             `json:"requestId,omitempty"`
	Operation security.Operation `json:"operation,omitempty"`
	Packages  []string           `json:"packages,omitempty"`
}

type AuditSink interface{ Record(AuditEvent) }

type JSONAuditSink struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func NewJSONAuditSink(writer io.Writer) *JSONAuditSink {
	return &JSONAuditSink{encoder: json.NewEncoder(writer)}
}

func (s *JSONAuditSink) Record(event AuditEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.encoder.Encode(event)
}

type discardAuditSink struct{}

func (discardAuditSink) Record(AuditEvent) {}
