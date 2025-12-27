package runner

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type mockNotifier struct {
	mu           sync.Mutex
	notifiedCount int
}

func (m *mockNotifier) Notify(ctx context.Context, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiedCount++
	return nil
}

func TestWorkerPool_Notifier(t *testing.T) {
	pool := NewWorkerPool(2)
	notifier := &mockNotifier{}
	pool.SetNotifier(notifier)
	pool.Start()

	pool.Submit(func(id int) error {
		return nil
	})
	pool.Submit(func(id int) error {
		return nil
	})

	pool.Stop()

	if notifier.notifiedCount != 2 {
		t.Errorf("expected 2 notifications, got %d", notifier.notifiedCount)
	}
}

func TestWorkerPool_Concurrency(t *testing.T) {
	numWorkers := 5
	pool := NewWorkerPool(numWorkers)
	pool.Start()

	numTasks := 10
	var mu sync.Mutex
	results := make(map[int]int) // workerID -> taskCount

	for i := 0; i < numTasks; i++ {
		pool.Submit(func(workerID int) error {
			mu.Lock()
			results[workerID]++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond) // Simulate work
			return nil
		})
	}

	pool.Stop()

	// Verify that tasks were distributed
	// Ideally, with 10 tasks and 5 workers, most workers should have done something.
	activeWorkers := 0
	for _, count := range results {
		if count > 0 {
			activeWorkers++
		}
	}

	if activeWorkers < 2 {
		t.Errorf("Expected tasks to be distributed among multiple workers, but only %d were active", activeWorkers)
	}
	
	totalTasks := 0
	for _, count := range results {
		totalTasks += count
	}
	if totalTasks != numTasks {
		t.Errorf("Expected %d tasks completed, got %d", numTasks, totalTasks)
	}
}

func TestWorkerPool_ErrorHandling(t *testing.T) {
	pool := NewWorkerPool(1)
	pool.Start()
	
	pool.Submit(func(id int) error {
		return fmt.Errorf("simulated error")
	})
	
	pool.Stop()
	// Just ensuring it doesn't crash/panic on error return
}

// TestWorkerPool_TaskDistribution verifies Feature #24: independent tasks are distributed to different workers
func TestWorkerPool_TaskDistribution(t *testing.T) {
	// Step 1: Submit 2 mock tasks to the queue
	pool := NewWorkerPool(3) // Use 3 workers to ensure distribution
	pool.Start()

	var mu sync.Mutex
	workerIDs := make([]int, 0, 2) // Track which worker IDs handle the tasks
	taskCompleted := make([]bool, 2) // Track if tasks completed successfully

	// Submit first task
	pool.Submit(func(workerID int) error {
		mu.Lock()
		workerIDs = append(workerIDs, workerID)
		taskCompleted[0] = true
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})

	// Submit second task
	pool.Submit(func(workerID int) error {
		mu.Lock()
		workerIDs = append(workerIDs, workerID)
		taskCompleted[1] = true
		mu.Unlock()
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	})

	pool.Stop()

	// Step 2: Verify logs show two different worker IDs handling them
	mu.Lock()
	defer mu.Unlock()

	if len(workerIDs) != 2 {
		t.Fatalf("Expected 2 tasks to be handled, got %d", len(workerIDs))
	}

	if workerIDs[0] == workerIDs[1] {
		t.Errorf("Expected tasks to be handled by different workers, but both were handled by worker %d", workerIDs[0])
	}

	// Step 3: Verify tasks complete successfully
	if !taskCompleted[0] {
		t.Error("Expected first task to complete successfully")
	}
	if !taskCompleted[1] {
		t.Error("Expected second task to complete successfully")
	}
}
