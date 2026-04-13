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
type Rule struct {
	PolicyID string   `json:"policy_id"`
	AgentID  string   `json:"agent_id"` // "*" matches any agent
	Domains  []string `json:"domains"`  // patterns; see matchDomain
	Action   string   `json:"action"`   // "allow" | "deny"
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

// Evaluate returns a Decision for the given (agentID, destHost) pair.
// destHost may include a port (e.g. "example.com:443"); the port is stripped before matching.
// If no rule matches the default action is deny.
func (e *Engine) Evaluate(agentID, destHost string) Decision {
	host := stripPort(destHost)

	e.mu.RLock()
	rules := e.rules
	e.mu.RUnlock()

	for _, r := range rules {
		if r.AgentID != "*" && r.AgentID != agentID {
			continue
		}
		for _, d := range r.Domains {
			if matchDomain(host, d) {
				return Decision{
					Action:   r.Action,
					PolicyID: r.PolicyID,
					Reason:   "matched rule " + r.PolicyID,
				}
			}
		}
	}
	return Decision{
		Action:   "deny",
		PolicyID: "default-deny",
		Reason:   "no matching allow rule",
	}
}

// matchDomain reports whether host matches a domain pattern.
//
//	"*"            — matches any host
//	"*.example.com" — matches example.com and any subdomain
//	"example.com"  — exact match only
func matchDomain(host, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // ".example.com"
		return host == pattern[2:] || strings.HasSuffix(host, suffix)
	}
	return host == pattern
}

func stripPort(hostport string) string {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}
