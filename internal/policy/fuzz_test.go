package policy

import "testing"

// FuzzEvaluateRich throws random agent/destination/method/path at the policy engine.
// Must never panic.
func FuzzEvaluateRich(f *testing.F) {
	f.Add("agent-1", "example.com:443", "GET", "/api/v1/data", "prod", "ops")
	f.Add("", "", "", "", "", "")
	f.Add("*", "*", "CONNECT", "", "", "")
	f.Add("agent\x00null", "evil.com\x00hidden.com", "GET", "/\x00/admin", "", "")
	f.Add("a", string(make([]byte, 10000)), "POST", "/", "dev", "team")

	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "allow-local", AgentID: "agent-1", Domains: []string{"localhost", "127.0.0.1"}, Methods: []string{"GET"}, Action: "allow"},
		{PolicyID: "deny-evil", AgentID: "*", Domains: []string{"*.evil.com"}, Action: "deny"},
		{PolicyID: "cond-prod", AgentID: "*", Domains: []string{"*"}, Conditions: map[string]string{"environment": "prod"}, Action: "allow"},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	f.Fuzz(func(t *testing.T, agentID, dest, method, path, env, team string) {
		// Must not panic. Any Decision is valid.
		d := eng.EvaluateRich(RequestContext{
			AgentID:     agentID,
			Destination: dest,
			Method:      method,
			Path:        path,
			Environment: env,
			TeamID:      team,
		})
		if d.Action != "allow" && d.Action != "deny" {
			t.Fatalf("unexpected action: %q", d.Action)
		}
	})
}

// FuzzMatchDomain throws random host/pattern pairs at the domain matcher.
func FuzzMatchDomain(f *testing.F) {
	f.Add("example.com", "example.com")
	f.Add("sub.example.com", "*.example.com")
	f.Add("", "*")
	f.Add("evil.com\x00good.com", "evil.com")
	f.Add(string(make([]byte, 5000)), "*.test")

	f.Fuzz(func(t *testing.T, host, pattern string) {
		_ = matchDomain(host, pattern)
	})
}
