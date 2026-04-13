package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Agent represents a registered agent identity.
type Agent struct {
	AgentID     string `json:"agent_id"`
	TeamID      string `json:"team_id"`
	ProjectID   string `json:"project_id"`
	Environment string `json:"environment"`
	APIKey      string `json:"api_key"`
	Status      string `json:"status"` // "active" | "disabled"
}

// Registry holds agent records indexed by API key and agent ID.
// All methods are safe for concurrent use.
type Registry struct {
	mu    sync.RWMutex
	byKey map[string]*Agent
	byID  map[string]*Agent
	path  string
}

// NewRegistry loads the registry from path. A missing file starts an empty registry.
func NewRegistry(path string) (*Registry, error) {
	r := &Registry{path: path}
	if err := r.Load(); err != nil {
		return nil, err
	}
	return r, nil
}

// Load reads the agent list from disk atomically.
// Safe to call from a SIGHUP handler while the proxy is running.
func (r *Registry) Load() error {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		r.mu.Lock()
		r.byKey = make(map[string]*Agent)
		r.byID = make(map[string]*Agent)
		r.mu.Unlock()
		return nil
	}
	if err != nil {
		return fmt.Errorf("read registry %s: %w", r.path, err)
	}

	var agents []Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		return fmt.Errorf("parse registry %s: %w", r.path, err)
	}

	byKey := make(map[string]*Agent, len(agents))
	byID := make(map[string]*Agent, len(agents))
	for i := range agents {
		a := &agents[i]
		byKey[a.APIKey] = a
		byID[a.AgentID] = a
	}

	r.mu.Lock()
	r.byKey = byKey
	r.byID = byID
	r.mu.Unlock()
	return nil
}

// LookupByKey resolves an agent from its API key.
// Returns nil if the key is unknown or the agent is not active.
func (r *Registry) LookupByKey(key string) *Agent {
	r.mu.RLock()
	a := r.byKey[key]
	r.mu.RUnlock()
	if a == nil || a.Status != "active" {
		return nil
	}
	return a
}

// All returns a snapshot of all registered agents (any status).
func (r *Registry) All() []Agent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Agent, 0, len(r.byID))
	for _, a := range r.byID {
		out = append(out, *a)
	}
	return out
}

// LookupByID returns the agent with the given ID, or nil if not found.
func (r *Registry) LookupByID(id string) *Agent {
	r.mu.RLock()
	a := r.byID[id]
	r.mu.RUnlock()
	if a == nil {
		return nil
	}
	cp := *a
	return &cp
}

// Add inserts or replaces an agent. Call Save() to persist.
func (r *Registry) Add(a Agent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := a
	r.byKey[cp.APIKey] = &cp
	r.byID[cp.AgentID] = &cp
}

// Remove deletes an agent by ID. Returns true if it existed. Call Save() to persist.
func (r *Registry) Remove(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.byID[id]
	if !ok {
		return false
	}
	delete(r.byID, id)
	delete(r.byKey, a.APIKey)
	return true
}

// Save writes the current agent list to disk atomically.
func (r *Registry) Save() error {
	r.mu.RLock()
	agents := make([]Agent, 0, len(r.byID))
	for _, a := range r.byID {
		agents = append(agents, *a)
	}
	r.mu.RUnlock()

	data, err := json.MarshalIndent(agents, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal agents: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write agents tmp: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("rename agents: %w", err)
	}
	return nil
}
