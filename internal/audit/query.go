package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// Filter controls which events are returned by Query.
type Filter struct {
	AgentID  string // empty = all agents
	Decision string // empty = all decisions
	Since    string // RFC3339 — events with Timestamp >= Since
	Limit    int    // 0 = unlimited
}

// Query reads the JSONL audit log at path and returns events matching f.
// Events are returned in file order (oldest first). If f.Limit > 0, only
// the last f.Limit matching events are returned.
func Query(path string, f Filter) ([]Event, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var sinceT time.Time
	if f.Since != "" {
		t, err := time.Parse(time.RFC3339, f.Since)
		if err == nil {
			sinceT = t
		}
	}

	var matched []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // skip malformed lines
		}
		if f.AgentID != "" && e.AgentID != f.AgentID {
			continue
		}
		if f.Decision != "" && e.Decision != f.Decision {
			continue
		}
		if !sinceT.IsZero() && e.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, e.Timestamp); err == nil {
				if t.Before(sinceT) {
					continue
				}
			}
		}
		matched = append(matched, e)
	}

	// Tail: return only the last N matching events.
	if f.Limit > 0 && len(matched) > f.Limit {
		matched = matched[len(matched)-f.Limit:]
	}
	return matched, scanner.Err()
}
