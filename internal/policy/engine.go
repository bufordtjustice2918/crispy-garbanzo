package policy

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
)

// Rule defines a policy entry. Rules are evaluated in slice order; first match wins.
// All match fields are optional — empty/nil means "match any".
type Rule struct {
	PolicyID     string            `json:"policy_id"`
	AgentID      string            `json:"agent_id"`                // "*" or empty matches any agent
	Domains      []string          `json:"domains"`                 // domain patterns; see matchDomain
	Methods      []string          `json:"methods,omitempty"`       // HTTP methods (GET, CONNECT, etc); empty = any
	PathPrefixes []string          `json:"path_prefixes,omitempty"` // path prefix match; empty = any
	Conditions   map[string]string `json:"conditions,omitempty"`    // key-value conditions (e.g. "environment":"prod")
	Action       string            `json:"action"`                  // "allow" | "deny"
}

// RequestContext carries per-request metadata for rich policy evaluation.
type RequestContext struct {
	AgentID     string
	Destination string // host or host:port
	Method      string // HTTP method
	Path        string // request path (for plain HTTP)
	Environment string // from identity
	TeamID      string // from identity
	ProjectID   string // from identity
}

// Decision is the result of evaluating a single request.
type Decision struct {
	Action   string // "allow" | "deny"
	PolicyID string
	Reason   string
}

// Engine evaluates policy rules against (agentID, destHost) pairs.
// All methods are safe for concurrent use.
type Engine struct {
	mu    sync.RWMutex
	rules []Rule
	path  string
}

// NewEngine loads policy from path. A missing file starts with no rules (default-deny).
func NewEngine(path string) (*Engine, error) {
	e := &Engine{path: path}
	if err := e.Load(); err != nil {
		return nil, err
	}
	return e, nil
}

// Load reads policy rules from disk atomically.
func (e *Engine) Load() error {
	data, err := os.ReadFile(e.path)
	if os.IsNotExist(err) {
		e.mu.Lock()
		e.rules = nil
		e.mu.Unlock()
		return nil
	}
	if err != nil {
		return fmt.Errorf("read policy %s: %w", e.path, err)
	}
	var rules []Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return fmt.Errorf("parse policy %s: %w", e.path, err)
	}
	e.mu.Lock()
	e.rules = rules
	e.mu.Unlock()
	return nil
}

// Rules returns a snapshot of the loaded rules.
func (e *Engine) Rules() []Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	return out
}

// LookupByID returns the rule with the given PolicyID, or nil if not found.
func (e *Engine) LookupByID(id string) *Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for i := range e.rules {
		if e.rules[i].PolicyID == id {
			cp := e.rules[i]
			return &cp
		}
	}
	return nil
}

// Add appends a rule. Call Save() to persist.
func (e *Engine) Add(r Rule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	// Replace if policy_id already exists.
	for i := range e.rules {
		if e.rules[i].PolicyID == r.PolicyID {
			e.rules[i] = r
			return
		}
	}
	e.rules = append(e.rules, r)
}

// Remove deletes a rule by PolicyID. Returns true if it existed. Call Save() to persist.
func (e *Engine) Remove(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.rules {
		if e.rules[i].PolicyID == id {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return true
		}
	}
	return false
}

// Save writes the current rules to disk atomically.
func (e *Engine) Save() error {
	e.mu.RLock()
	out := make([]Rule, len(e.rules))
	copy(out, e.rules)
	e.mu.RUnlock()

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	tmp := e.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write policy tmp: %w", err)
	}
	if err := os.Rename(tmp, e.path); err != nil {
		return fmt.Errorf("rename policy: %w", err)
	}
	return nil
}

// Evaluate returns a Decision for the given (agentID, destHost) pair.
// This is the simple API — for method/path/condition matching, use EvaluateRich.
func (e *Engine) Evaluate(agentID, destHost string) Decision {
	return e.EvaluateRich(RequestContext{
		AgentID:     agentID,
		Destination: destHost,
	})
}

// EvaluateRich returns a Decision using full request context including method,
// path, and identity conditions. Empty context fields match any rule field.
// Rules are evaluated in order; first match wins. Default action is deny.
func (e *Engine) EvaluateRich(ctx RequestContext) Decision {
	host := sanitizeHost(stripPort(ctx.Destination))

	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	for _, r := range rules {
		if r.AgentID != "*" && r.AgentID != "" && r.AgentID != ctx.AgentID {
			continue
		}
		if !matchDomainList(host, r.Domains) {
			continue
		}
		if len(r.Methods) > 0 && ctx.Method != "" && !containsIgnoreCase(r.Methods, ctx.Method) {
			continue
		}
		if len(r.PathPrefixes) > 0 && ctx.Path != "" && !matchAnyPrefix(ctx.Path, r.PathPrefixes) {
			continue
		}
		if len(r.Conditions) > 0 && !matchConditions(r.Conditions, ctx) {
			continue
		}
		return Decision{
			Action:   r.Action,
			PolicyID: r.PolicyID,
			Reason:   "matched rule " + r.PolicyID,
		}
	}
	return Decision{
		Action:   "deny",
		PolicyID: "default-deny",
		Reason:   "no matching allow rule",
	}
}

func matchDomainList(host string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	for _, d := range domains {
		if matchDomain(host, d) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(list []string, val string) bool {
	val = strings.ToUpper(val)
	for _, s := range list {
		if strings.ToUpper(s) == val {
			return true
		}
	}
	return false
}

func matchAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func matchConditions(conds map[string]string, ctx RequestContext) bool {
	for k, v := range conds {
		switch k {
		case "environment":
			if ctx.Environment != "" && ctx.Environment != v {
				return false
			}
		case "team_id":
			if ctx.TeamID != "" && ctx.TeamID != v {
				return false
			}
		case "project_id":
			if ctx.ProjectID != "" && ctx.ProjectID != v {
				return false
			}
		}
	}
	return true
}

// matchDomain reports whether host matches a domain pattern.
//
//	"*"            — matches any host
//	"*.example.com" — matches example.com and any subdomain
//	"example.com"  — exact match only
func matchDomain(host, pattern string) bool {
	pattern = strings.ToLower(pattern)
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return host == pattern[2:] || strings.HasSuffix(host, suffix)
	}
	return host == pattern
}

// sanitizeHost removes null bytes, trailing dots, leading/trailing whitespace,
// and lowercases the hostname. This prevents bypass attacks via encoding tricks.
func sanitizeHost(host string) string {
	// Truncate at first null byte (prevent null-byte injection).
	if idx := strings.IndexByte(host, 0); idx >= 0 {
		host = host[:idx]
	}
	// Strip trailing dot (DNS canonical form).
	host = strings.TrimRight(host, ".")
	// Trim whitespace.
	host = strings.TrimSpace(host)
	// Lowercase for case-insensitive matching.
	host = strings.ToLower(host)
	return host
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}
