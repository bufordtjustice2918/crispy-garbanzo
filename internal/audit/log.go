package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event is one decision record written per proxy request.
type Event struct {
	Timestamp   string `json:"timestamp"`
	RequestID   string `json:"request_id"`
	AgentID     string `json:"agent_id"`
	TeamID      string `json:"team_id"`
	ProjectID   string `json:"project_id"`
	Environment string `json:"environment"`
	Destination string `json:"destination"`
	Method      string `json:"http_method"`
	Decision    string `json:"decision"`
	PolicyID    string `json:"policy_id"`
	LatencyMs   int64  `json:"latency_ms"`
	BytesOut    int64  `json:"bytes_out"`
}

// Log is an append-only JSONL file. One line per Event.
// All methods are safe for concurrent use.
type Log struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// NewLog opens (or creates) the JSONL file at path, creating parent directories as needed.
func NewLog(path string) (*Log, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create audit log dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open audit log %s: %w", path, err)
	}
	return &Log{path: path, f: f}, nil
}

// Write appends a single JSON line to the log.
// Timestamp is set to UTC now if empty.
func (l *Log) Write(e Event) error {
	if e.Timestamp == "" {
		e.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	l.mu.Lock()
	_, err = fmt.Fprintf(l.f, "%s\n", data)
	l.mu.Unlock()
	return err
}

// Close flushes and closes the underlying file.
func (l *Log) Close() error {
	return l.f.Close()
}
