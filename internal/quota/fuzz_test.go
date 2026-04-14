package quota

import "testing"

// FuzzCheck throws random agent IDs at the limiter. Must never panic.
func FuzzCheck(f *testing.F) {
	f.Add("agent-1")
	f.Add("")
	f.Add(string(make([]byte, 10000)))
	f.Add("agent\x00null")
	f.Add("*")

	lim, _ := NewLimiter("/nonexistent")
	lim.Set(Limit{AgentID: "agent-1", RPS: 100, Mode: "hard_stop"})
	lim.Set(Limit{AgentID: "*", RPS: 50, Mode: "alert_only"})

	f.Fuzz(func(t *testing.T, agentID string) {
		d := lim.Check(agentID)
		if d.Mode != "" && d.Mode != "hard_stop" && d.Mode != "alert_only" {
			t.Fatalf("unexpected mode: %q", d.Mode)
		}
	})
}
