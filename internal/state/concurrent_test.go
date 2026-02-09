package state

import (
	"fmt"
	"sync"
	"testing"
)

func TestConcurrentSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	// Seed with an initial state
	initial := NewState()
	initial.Frappe = FrappeState{Version: "15", Database: "mariadb"}
	if err := initial.Save(tmpDir); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}

	const writers = 5
	const readers = 10
	const iterations = 20

	var wg sync.WaitGroup

	// Concurrent writers
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				s := NewState()
				s.Frappe = FrappeState{
					Version:  "15",
					Database: "mariadb",
				}
				s.AddApp(AppState{
					Name:   fmt.Sprintf("app-%d-%d", id, i),
					Branch: "main",
				})
				s.AddSite(SiteState{
					Name: fmt.Sprintf("site-%d-%d.localhost", id, i),
					Apps: []string{"frappe"},
				})
				if err := s.Save(tmpDir); err != nil {
					t.Errorf("writer %d iteration %d: save failed: %v", id, i, err)
					return
				}
			}
		}(w)
	}

	// Concurrent readers
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				s, err := Load(tmpDir)
				if err != nil {
					t.Errorf("reader %d iteration %d: load failed: %v", id, i, err)
					return
				}
				// Access maps to verify no corruption
				_ = s.AppNames()
				_ = s.SiteNames()
				_ = s.GetDefaultSite()
				_ = s.IsEmpty()
			}
		}(r)
	}

	wg.Wait()

	// Final load should succeed and be valid
	final, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("final load failed: %v", err)
	}
	if final.Version != StateVersion {
		t.Errorf("final state version = %q, want %q", final.Version, StateVersion)
	}
}
