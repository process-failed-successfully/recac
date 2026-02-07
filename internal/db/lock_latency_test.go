package db

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestAcquireLockLatency verifies that acquiring a lock is responsive.
// It ensures that after a lock is released, a waiting acquirer picks it up quickly.
func TestAcquireLockLatency(t *testing.T) {
	// Create a temporary database file
	tmpDir, err := os.MkdirTemp("", "lock_perf_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "lock_perf.db")
	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	projectID := "proj_lock_perf"
	path := "critical_resource.txt"
	agent1 := "agent1"
	agent2 := "agent2"

	// Agent 1 acquires the lock
	acquired, err := store.AcquireLock(projectID, path, agent1, 1*time.Second)
	if err != nil || !acquired {
		t.Fatalf("Agent 1 failed to acquire lock: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	// Agent 2 tries to acquire the lock in a goroutine
	go func() {
		defer wg.Done()
		// Agent 2 should block until Agent 1 releases
		acquired, err := store.AcquireLock(projectID, path, agent2, 5*time.Second)
		if err != nil || !acquired {
			t.Errorf("Agent 2 failed to acquire lock: %v", err)
		}
	}()

	// Ensure Agent 2 has had time to fail once and enter sleep loop.
	// We wait 100ms to be sure it's sleeping.
	time.Sleep(100 * time.Millisecond)

	startRelease := time.Now()
	// Release lock
	err = store.ReleaseLock(projectID, path, agent1)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	wg.Wait()
	duration := time.Since(startRelease)

	t.Logf("Wait time after release: %v", duration)

	// We expect the latency to be reasonably low.
	// With 50ms polling, latency should be on average 25ms + overhead.
	// We set a threshold of 150ms to be safe (CI environments can be slow).
	// If it was 500ms polling, this would be ~400ms on average.
	threshold := 150 * time.Millisecond
	if duration > threshold {
		t.Errorf("Performance Regression: High latency (%v) detected. Expected < %v.", duration, threshold)
	} else {
		t.Logf("Performance Observation: Low latency (%v) detected.", duration)
	}
}
