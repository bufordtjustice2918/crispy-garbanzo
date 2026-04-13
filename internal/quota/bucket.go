// Package quota implements per-agent rate limiting using a token bucket algorithm.
//
// Each agent can have independent RPS (requests per second) and RPM (requests per minute)
// limits. Two enforcement modes are supported:
//
//   - "hard_stop": reject the request with 429 when the bucket is empty
//   - "alert_only": log the overage but allow the request through
package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Limit defines rate limits for a single agent.
type Limit struct {
	AgentID string  `json:"agent_id"`
	RPS     float64 `json:"rps"`  // requests per second (0 = unlimited)
	RPM     float64 `json:"rpm"`  // requests per minute (0 = unlimited)
	Mode    string  `json:"mode"` // "hard_stop" | "alert_only"
}

// Decision is the result of a quota check.
type Decision struct {
	Allowed bool
	Mode    string // "hard_stop" | "alert_only" | "" (no limit)
	Reason  string
}

// bucket is an internal token bucket.
type bucket struct {
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	last     time.Time
}

func (b *bucket) allow(now time.Time) bool {
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.last = now
	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}

// agentBuckets holds per-second and per-minute buckets for one agent.
type agentBuckets struct {
	rps *bucket
	rpm *bucket
}

// Limiter enforces per-agent rate limits.
// All methods are safe for concurrent use.
type Limiter struct {
	mu      sync.Mutex
	limits  map[string]*Limit        // agent_id -> Limit
	buckets map[string]*agentBuckets // agent_id -> buckets
	path    string
}

// NewLimiter loads quota config from path. A missing file means no limits.
func NewLimiter(path string) (*Limiter, error) {
	l := &Limiter{
		path:    path,
		limits:  make(map[string]*Limit),
		buckets: make(map[string]*agentBuckets),
	}
	if err := l.Load(); err != nil {
		return nil, err
	}
	return l, nil
}

// Load reads quota config from disk. Safe to call while serving.
func (l *Limiter) Load() error {
	data, err := os.ReadFile(l.path)
	if os.IsNotExist(err) {
		l.mu.Lock()
		l.limits = make(map[string]*Limit)
		l.buckets = make(map[string]*agentBuckets)
		l.mu.Unlock()
		return nil
	}
	if err != nil {
		return fmt.Errorf("read quotas %s: %w", l.path, err)
	}

	var limits []Limit
	if err := json.Unmarshal(data, &limits); err != nil {
		return fmt.Errorf("parse quotas %s: %w", l.path, err)
	}

	newLimits := make(map[string]*Limit, len(limits))
	newBuckets := make(map[string]*agentBuckets, len(limits))
	now := time.Now()

	for i := range limits {
		lim := &limits[i]
		if lim.Mode == "" {
			lim.Mode = "hard_stop"
		}
		newLimits[lim.AgentID] = lim
		ab := &agentBuckets{}
		if lim.RPS > 0 {
			ab.rps = &bucket{tokens: lim.RPS, capacity: lim.RPS, rate: lim.RPS, last: now}
		}
		if lim.RPM > 0 {
			rpmRate := lim.RPM / 60.0
			ab.rpm = &bucket{tokens: lim.RPM, capacity: lim.RPM, rate: rpmRate, last: now}
		}
		newBuckets[lim.AgentID] = ab
	}

	l.mu.Lock()
	l.limits = newLimits
	l.buckets = newBuckets
	l.mu.Unlock()
	return nil
}

// Check evaluates whether a request from agentID should proceed.
func (l *Limiter) Check(agentID string) Decision {
	l.mu.Lock()
	defer l.mu.Unlock()

	lim, ok := l.limits[agentID]
	if !ok {
		return Decision{Allowed: true}
	}

	ab := l.buckets[agentID]
	if ab == nil {
		return Decision{Allowed: true}
	}

	now := time.Now()

	if ab.rps != nil && !ab.rps.allow(now) {
		return Decision{
			Allowed: lim.Mode == "alert_only",
			Mode:    lim.Mode,
			Reason:  fmt.Sprintf("RPS limit exceeded (%.0f rps)", lim.RPS),
		}
	}
	if ab.rpm != nil && !ab.rpm.allow(now) {
		return Decision{
			Allowed: lim.Mode == "alert_only",
			Mode:    lim.Mode,
			Reason:  fmt.Sprintf("RPM limit exceeded (%.0f rpm)", lim.RPM),
		}
	}

	return Decision{Allowed: true, Mode: lim.Mode}
}

// All returns a snapshot of all configured limits.
func (l *Limiter) All() []Limit {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Limit, 0, len(l.limits))
	for _, lim := range l.limits {
		out = append(out, *lim)
	}
	return out
}

// LookupByID returns the limit for the given agent, or nil if not found.
func (l *Limiter) LookupByID(agentID string) *Limit {
	l.mu.Lock()
	defer l.mu.Unlock()
	lim, ok := l.limits[agentID]
	if !ok {
		return nil
	}
	cp := *lim
	return &cp
}

// Set adds or replaces a limit for an agent. Call Save() to persist.
// Resets the token buckets for this agent.
func (l *Limiter) Set(lim Limit) {
	if lim.Mode == "" {
		lim.Mode = "hard_stop"
	}
	now := time.Now()
	ab := &agentBuckets{}
	if lim.RPS > 0 {
		ab.rps = &bucket{tokens: lim.RPS, capacity: lim.RPS, rate: lim.RPS, last: now}
	}
	if lim.RPM > 0 {
		rpmRate := lim.RPM / 60.0
		ab.rpm = &bucket{tokens: lim.RPM, capacity: lim.RPM, rate: rpmRate, last: now}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	cp := lim
	l.limits[lim.AgentID] = &cp
	l.buckets[lim.AgentID] = ab
}

// Remove deletes a limit for an agent. Returns true if it existed.
func (l *Limiter) Remove(agentID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.limits[agentID]
	if !ok {
		return false
	}
	delete(l.limits, agentID)
	delete(l.buckets, agentID)
	return true
}

// Save writes the current limits to disk atomically.
func (l *Limiter) Save() error {
	l.mu.Lock()
	out := make([]Limit, 0, len(l.limits))
	for _, lim := range l.limits {
		out = append(out, *lim)
	}
	l.mu.Unlock()

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal quotas: %w", err)
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write quotas tmp: %w", err)
	}
	if err := os.Rename(tmp, l.path); err != nil {
		return fmt.Errorf("rename quotas: %w", err)
	}
	return nil
}
