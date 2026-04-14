package policy

import (
	"sync"
	"testing"
)

// TestConcurrentEvaluate hammers the policy engine from 100 goroutines.
// Run with -race to detect data races.
func TestConcurrentEvaluate(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "allow-local", AgentID: "a1", Domains: []string{"localhost"}, Action: "allow"},
		{PolicyID: "deny-evil", AgentID: "*", Domains: []string{"*.evil.com"}, Action: "deny"},
		{PolicyID: "default-deny", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	var wg sync.WaitGroup
	errs := make(chan string, 1000)

	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				d := eng.Evaluate("a1", "localhost")
				if d.Action != "allow" {
					errs <- "expected allow for localhost, got " + d.Action
					return
				}
				d = eng.Evaluate("a1", "sub.evil.com")
				if d.Action != "deny" {
					errs <- "expected deny for evil.com, got " + d.Action
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

// TestConcurrentReadWrite tests concurrent reads and writes (Add/Remove during Evaluate).
func TestConcurrentReadWrite(t *testing.T) {
	eng := &Engine{}
	eng.rules = []Rule{
		{PolicyID: "base", AgentID: "*", Domains: []string{"*"}, Action: "deny"},
	}

	var wg sync.WaitGroup

	// 50 readers.
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 500 {
				eng.Evaluate("a1", "example.com")
				eng.Rules()
			}
		}()
	}

	// 10 writers.
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := "dynamic-" + string(rune('a'+i))
			for range 100 {
				eng.Add(Rule{PolicyID: id, AgentID: "*", Domains: []string{"dyn.com"}, Action: "allow"})
				eng.Remove(id)
			}
		}()
	}

	wg.Wait()
}
