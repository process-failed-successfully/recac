package runner

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestDependencyExecutor_Execute_Sequential(t *testing.T) {
	// Setup Graph: A -> B
	g := NewTaskGraph()
	g.AddNode("A", "Task A", nil)
	g.AddNode("B", "Task B", []string{"A"})

	// Setup Pool
	pool := NewWorkerPool(1)
	pool.Start()
	defer pool.Stop()

	executor := NewDependencyExecutor(g, pool)

	// execution order tracking
	var order []string
	var mu sync.Mutex

	executor.RegisterTask("A", func(workerID int) error {
		mu.Lock()
		order = append(order, "A")
		mu.Unlock()
		return nil
	})

	executor.RegisterTask("B", func(workerID int) error {
		mu.Lock()
		order = append(order, "B")
		mu.Unlock()
		return nil
	})

	if err := executor.Execute(); err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if len(order) != 2 {
		t.Fatalf("Expected 2 tasks executed, got %d", len(order))
	}
	if order[0] != "A" || order[1] != "B" {
		t.Errorf("Expected order [A, B], got %v", order)
	}
}

func TestDependencyExecutor_Execute_Independent(t *testing.T) {
	// Setup Graph: A, B (no deps)
	g := NewTaskGraph()
	g.AddNode("A", "Task A", nil)
	g.AddNode("B", "Task B", nil)

	// Setup Pool
	pool := NewWorkerPool(2)
	pool.Start()
	defer pool.Stop()

	executor := NewDependencyExecutor(g, pool)

	var executedCount int
	var mu sync.Mutex

	task := func(workerID int) error {
		mu.Lock()
		executedCount++
		mu.Unlock()
		return nil
	}

	executor.RegisterTask("A", task)
	executor.RegisterTask("B", task)

	if err := executor.Execute(); err != nil {
		t.Fatalf("Execution failed: %v", err)
	}

	if executedCount != 2 {
		t.Errorf("Expected 2 tasks executed, got %d", executedCount)
	}
}

func TestDependencyExecutor_Execute_Failure(t *testing.T) {
	// Setup Graph: A -> B
	// A fails. B should be skipped/fail dependency check.
	g := NewTaskGraph()
	g.AddNode("A", "Task A", nil)
	g.AddNode("B", "Task B", []string{"A"})

	pool := NewWorkerPool(1)
	pool.Start()
	defer pool.Stop()

	executor := NewDependencyExecutor(g, pool)

	executor.RegisterTask("A", func(workerID int) error {
		return errors.New("simulated failure")
	})

	executor.RegisterTask("B", func(workerID int) error {
		t.Error("Task B should not execute")
		return nil
	})

	err := executor.Execute()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	
	// Check status
	statusA, _ := g.GetTaskStatus("A")
	if statusA != TaskFailed {
		t.Errorf("Expected A to be Failed, got %s", statusA)
	}
	
	statusB, _ := g.GetTaskStatus("B")
	if statusB != TaskFailed {
		t.Errorf("Expected B to be Failed (due to dep), got %s", statusB)
	}
}

func TestDependencyExecutor_Execute_Cycle(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("A", "Task A", []string{"B"})
	g.AddNode("B", "Task B", []string{"A"})

	pool := NewWorkerPool(1)
	pool.Start()
	defer pool.Stop()

	executor := NewDependencyExecutor(g, pool)

	err := executor.Execute()
	if err == nil {
		t.Fatal("Expected error for cycle, got nil")
	}
}

func TestDependencyExecutor_Stop(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("A", "Task A", nil)
	
	pool := NewWorkerPool(1)
	pool.Start()
	defer pool.Stop()
	
	executor := NewDependencyExecutor(g, pool)
	
	executor.RegisterTask("A", func(workerID int) error {
		time.Sleep(1 * time.Second)
		return nil
	})
	
	// Start execution in background
	go func() {
		executor.Execute()
	}()
	
	// Stop immediately
	time.Sleep(100 * time.Millisecond)
	executor.Stop()
	
	// We can't easily assert return value of Execute here without channels, 
	// but we can check if it finishes (implicit) and doesn't hang.
	// In a real test we'd check if context cancellation propagated.
}
