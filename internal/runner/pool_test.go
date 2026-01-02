package runner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type MockNotifier struct{}

func (m *MockNotifier) Notify(ctx context.Context, msg string) error {
	return nil
}

func TestWorkerPool(t *testing.T) {
	// 1. NewWorkerPool
	pool := NewWorkerPool(2)
	if pool.NumWorkers != 2 {
		t.Errorf("Expected 2 workers, got %d", pool.NumWorkers)
	}

	// 2. Notifier
	notifier := &MockNotifier{}
	pool.SetNotifier(notifier)
	if pool.GetNotifier() != notifier {
		t.Error("Notifier not set correctly")
	}

	// 3. Start and Submit
	pool.Start()

	var wg sync.WaitGroup
	wg.Add(2)

	task1 := func(id int) error {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		return nil
	}
	task2 := func(id int) error {
		defer wg.Done()
		return errors.New("simulated error")
	}

	pool.Submit(task1)
	pool.Submit(task2)

	// 4. Wait
	pool.Wait()
	wg.Wait() // Double check

	// 5. ActiveCount should be 0 (eventually)
	// Might be race if we check immediately after Wait? 
	// Wait() waits for taskWG. taskWG done in worker loop.
	// So ActiveCount decrement happens BEFORE taskWG.Done().
	// So ActiveCount should be 0.
	if count := pool.ActiveCount(); count != 0 {
		t.Errorf("Expected 0 active tasks, got %d", count)
	}

	// 6. Stop
	pool.Stop()
}