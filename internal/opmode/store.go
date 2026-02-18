package opmode

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ConfigureRequest struct {
	Actor   string         `json:"actor"`
	Changes map[string]any `json:"changes"`
}

type ConfigureResponse struct {
	RevisionID string    `json:"revision_id"`
	Status     string    `json:"status"`
	Validated  bool      `json:"validated"`
	Timestamp  time.Time `json:"timestamp"`
}

type CommitRequest struct {
	Actor              string `json:"actor"`
	ExpectedRevisionID string `json:"expected_revision_id,omitempty"`
}

type CommitResponse struct {
	CommitID   string    `json:"commit_id"`
	RevisionID string    `json:"revision_id"`
	Status     string    `json:"status"`
	Timestamp  time.Time `json:"timestamp"`
}

type Revision struct {
	RevisionID string         `json:"revision_id"`
	Actor      string         `json:"actor"`
	Changes    map[string]any `json:"changes"`
	CreatedAt  time.Time      `json:"created_at"`
}

type CommitRecord struct {
	CommitID   string    `json:"commit_id"`
	RevisionID string    `json:"revision_id"`
	Actor      string    `json:"actor"`
	Committed  time.Time `json:"committed_at"`
}

type StateView struct {
	Staged      *Revision     `json:"staged,omitempty"`
	Active      *Revision     `json:"active,omitempty"`
	LastCommit  *CommitRecord `json:"last_commit,omitempty"`
	HistorySize int           `json:"history_size"`
}

type Store struct {
	baseDir string
	mu      sync.Mutex
}

var ErrNoStagedRevision = errors.New("no staged revision")
var ErrRevisionMismatch = errors.New("expected revision does not match staged revision")

func NewStore(baseDir string) (*Store, error) {
	if baseDir == "" {
		baseDir = "state"
	}
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	return &Store{baseDir: baseDir}, nil
}

func (s *Store) Configure(req ConfigureRequest) (ConfigureResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	revisionID, err := newID("rev")
	if err != nil {
		return ConfigureResponse{}, err
	}

	rev := Revision{
		RevisionID: revisionID,
		Actor:      req.Actor,
		Changes:    req.Changes,
		CreatedAt:  now,
	}
	if err := writeJSONAtomic(s.path("staged.json"), rev); err != nil {
		return ConfigureResponse{}, err
	}

	if err := appendAudit(s.path("audit.log"), map[string]any{
		"event":       "configure",
		"actor":       req.Actor,
		"revision_id": revisionID,
		"timestamp":   now.Format(time.RFC3339Nano),
	}); err != nil {
		return ConfigureResponse{}, err
	}

	return ConfigureResponse{
		RevisionID: revisionID,
		Status:     "staged",
		Validated:  true,
		Timestamp:  now,
	}, nil
}

func (s *Store) Commit(req CommitRequest) (CommitResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var staged Revision
	if err := readJSON(s.path("staged.json"), &staged); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CommitResponse{}, ErrNoStagedRevision
		}
		return CommitResponse{}, err
	}

	if req.ExpectedRevisionID != "" && req.ExpectedRevisionID != staged.RevisionID {
		return CommitResponse{}, ErrRevisionMismatch
	}

	now := time.Now().UTC()
	commitID, err := newID("cmt")
	if err != nil {
		return CommitResponse{}, err
	}

	if err := writeJSONAtomic(s.path("active.json"), staged); err != nil {
		return CommitResponse{}, err
	}

	record := CommitRecord{
		CommitID:   commitID,
		RevisionID: staged.RevisionID,
		Actor:      req.Actor,
		Committed:  now,
	}
	if err := appendJSONLine(s.path("commits.jsonl"), record); err != nil {
		return CommitResponse{}, err
	}

	if err := appendAudit(s.path("audit.log"), map[string]any{
		"event":       "commit",
		"actor":       req.Actor,
		"commit_id":   commitID,
		"revision_id": staged.RevisionID,
		"timestamp":   now.Format(time.RFC3339Nano),
	}); err != nil {
		return CommitResponse{}, err
	}

	if err := os.Remove(s.path("staged.json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return CommitResponse{}, err
	}

	return CommitResponse{
		CommitID:   commitID,
		RevisionID: staged.RevisionID,
		Status:     "committed",
		Timestamp:  now,
	}, nil
}

func (s *Store) State() (StateView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out StateView
	var staged Revision
	if err := readJSON(s.path("staged.json"), &staged); err == nil {
		out.Staged = &staged
	}

	var active Revision
	if err := readJSON(s.path("active.json"), &active); err == nil {
		out.Active = &active
	}

	commits, err := readCommitHistory(s.path("commits.jsonl"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return StateView{}, err
		}
	}
	out.HistorySize = len(commits)
	if len(commits) > 0 {
		last := commits[len(commits)-1]
		out.LastCommit = &last
	}
	return out, nil
}

func (s *Store) path(name string) string {
	return filepath.Join(s.baseDir, name)
}

func newID(prefix string) (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random id: %w", err)
	}
	return fmt.Sprintf("%s_%d_%s", prefix, time.Now().UTC().Unix(), hex.EncodeToString(buf)), nil
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, dst any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

func appendJSONLine(path string, v any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	return enc.Encode(v)
}

func appendAudit(path string, v map[string]any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func readCommitHistory(path string) ([]CommitRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var out []CommitRecord
	lines := bytesSplitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var rec CommitRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func bytesSplitLines(data []byte) [][]byte {
	start := 0
	var out [][]byte
	for i, b := range data {
		if b == '\n' {
			out = append(out, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		out = append(out, data[start:])
	}
	return out
}
