package runner

import (
	"fmt"
	"path/filepath"
	"recac/internal/agent"
	"sync"
	"testing"
)

func TestRace_StateManager(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "test_state.json")
	sm := agent.NewStateManager(stateFile)
	var wg sync.WaitGroup

	numGoroutines := 20
	iterations := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Concurrent Save
				state, _ := sm.Load()
				state.Memory = append(state.Memory, fmt.Sprintf("msg from %d-%d", id, j))
				sm.Save(state)

				// Concurrent Load
				_, _ = sm.Load()
			}
		}(i)
	}

	wg.Wait()
}

func TestRace_WorkerPool_Notifier(t *testing.T) {
	// Testing if changing the Notifier while workers are running causes a race
	pool := NewWorkerPool(4)
	pool.Start()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			pool.Submit(func(id int) error {
				return nil
			})
		}
	}()

	// Concurrent update of Notifier
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			pool.SetNotifier(nil)
		}
	}()

	wg.Wait()
	pool.Stop()
}
