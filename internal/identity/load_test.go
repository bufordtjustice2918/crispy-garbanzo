package identity

import (
	"sync"
	"testing"
)

// TestConcurrentLookup hammers the registry from 100 goroutines.
func TestConcurrentLookup(t *testing.T) {
	reg := &Registry{
		byKey: make(map[string]*Agent),
		byID:  make(map[string]*Agent),
	}
	for i := range 100 {
		a := &Agent{
			AgentID: "agent-" + string(rune(i+48)),
			APIKey:  "key-" + string(rune(i+48)),
			Status:  "active",
		}
		reg.byKey[a.APIKey] = a
		reg.byID[a.AgentID] = a
	}

	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				reg.LookupByKey("key-0")
				reg.LookupByKey("nonexistent")
				reg.All()
			}
		}()
	}
	wg.Wait()
}

// TestConcurrentAddRemove tests concurrent Add/Remove/Lookup.
func TestConcurrentAddRemove(t *testing.T) {
	reg := &Registry{
		byKey: make(map[string]*Agent),
		byID:  make(map[string]*Agent),
		path:  "/nonexistent",
	}

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := "agent-" + string(rune(i+65))
			key := "key-" + string(rune(i+65))
			for range 200 {
				reg.Add(Agent{AgentID: id, APIKey: key, Status: "active"})
				reg.LookupByKey(key)
				reg.LookupByID(id)
				reg.Remove(id)
			}
		}()
	}
	wg.Wait()
}
