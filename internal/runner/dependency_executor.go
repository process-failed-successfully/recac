package runner

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DependencyExecutor executes tasks with dependency awareness
type DependencyExecutor struct {
	graph      *TaskGraph
	pool       *WorkerPool
	taskFuncs  map[string]Task // Map task ID to execution function
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewDependencyExecutor creates a new dependency-aware executor
func NewDependencyExecutor(graph *TaskGraph, pool *WorkerPool) *DependencyExecutor {
	ctx, cancel := context.WithCancel(context.Background())
	return &DependencyExecutor{
		graph:     graph,
		pool:      pool,
		taskFuncs: make(map[string]Task),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// RegisterTask registers a task function for a task ID
func (e *DependencyExecutor) RegisterTask(taskID string, taskFunc Task) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.taskFuncs[taskID] = taskFunc
}

// Execute runs all tasks respecting dependencies
func (e *DependencyExecutor) Execute() error {
	// First, check for circular dependencies
	if cycle, err := e.graph.DetectCycles(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	} else if cycle != nil {
		return fmt.Errorf("circular dependency detected: %v", cycle)
	}

	// Get topological order
	executionOrder, err := e.graph.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to determine execution order: %w", err)
	}

	fmt.Printf("Task execution order (topological sort): %v\n", executionOrder)

	// Track task completion
	var wg sync.WaitGroup
	taskStatus := make(map[string]bool) // true = done or failed
	taskSubmitted := make(map[string]bool) // true = already submitted
	var statusMu sync.Mutex
	executionErrors := make(map[string]error)

	// Function to check if a task is ready and submit it
	checkAndSubmit := func(taskID string) {
		statusMu.Lock()
		// Check if already processed or submitted
		if taskStatus[taskID] || taskSubmitted[taskID] {
			statusMu.Unlock()
			return
		}
		
		// Check if all dependencies are satisfied (while holding lock)
		node, err := e.graph.GetTask(taskID)
		if err != nil {
			executionErrors[taskID] = err
			taskStatus[taskID] = true
			statusMu.Unlock()
			_ = e.graph.MarkTaskStatus(taskID, TaskFailed, err)
			return
		}

		allDepsDone := true
		for _, depID := range node.Dependencies {
			if !taskStatus[depID] {
				allDepsDone = false
				break
			}
			// Check if dependency failed
			if depErr, exists := executionErrors[depID]; exists {
				// Dependency failed, mark this task as failed
				err := fmt.Errorf("dependency %s failed: %w", depID, depErr)
				executionErrors[taskID] = err
				taskStatus[taskID] = true
				statusMu.Unlock()
				_ = e.graph.MarkTaskStatus(taskID, TaskFailed, err)
				return
			}
		}

		if !allDepsDone {
			statusMu.Unlock()
			return // Not ready yet
		}

		// Mark as submitted to prevent duplicate submissions (atomically)
		taskSubmitted[taskID] = true
		statusMu.Unlock()

		// Mark as in progress
		_ = e.graph.MarkTaskStatus(taskID, TaskInProgress, nil)

		// Get task function
		e.mu.RLock()
		taskFunc, exists := e.taskFuncs[taskID]
		e.mu.RUnlock()

		if !exists {
			// No task function registered, mark as done (skip)
			statusMu.Lock()
			taskStatus[taskID] = true
			statusMu.Unlock()
			_ = e.graph.MarkTaskStatus(taskID, TaskDone, nil)
			return
		}

		// Submit to worker pool (in goroutine to avoid blocking)
		wg.Add(1)
		go func() {
			// Use select to handle closed channel gracefully
			select {
			case <-e.ctx.Done():
				statusMu.Lock()
				taskStatus[taskID] = true
				statusMu.Unlock()
				wg.Done()
				return
			default:
				// Try to submit, but handle potential channel closure
				func() {
					defer func() {
						if r := recover(); r != nil {
							// Channel closed, mark task as failed
							statusMu.Lock()
							taskStatus[taskID] = true
							executionErrors[taskID] = fmt.Errorf("worker pool closed")
							statusMu.Unlock()
							_ = e.graph.MarkTaskStatus(taskID, TaskFailed, fmt.Errorf("worker pool closed"))
							wg.Done()
						}
					}()
					e.pool.Submit(func(workerID int) error {
				defer wg.Done()

				fmt.Printf("Task %s: Executing on Worker %d\n", taskID, workerID)
				
				// Execute task
				err := taskFunc(workerID)
				
				statusMu.Lock()
				defer statusMu.Unlock()

				if err != nil {
					executionErrors[taskID] = err
					taskStatus[taskID] = true
					_ = e.graph.MarkTaskStatus(taskID, TaskFailed, err)
					return err
				}

				taskStatus[taskID] = true
				_ = e.graph.MarkTaskStatus(taskID, TaskDone, nil)
				fmt.Printf("Task %s: Completed successfully\n", taskID)
				return nil
					})
				}()
			}
		}()
	}

	// Submit initial ready tasks
	for _, taskID := range executionOrder {
		checkAndSubmit(taskID)
	}

	// Monitor and submit tasks as dependencies complete
	done := make(chan bool, 1) // Buffered to prevent blocking
	monitorDone := make(chan bool, 1)
	go func() {
		defer func() {
			monitorDone <- true
		}()
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-e.ctx.Done():
				return
			case <-ticker.C:
				statusMu.Lock()
				allDone := len(taskStatus) == len(executionOrder)
				statusMu.Unlock()

				if allDone {
					select {
					case done <- true:
					default:
					}
					return
				}

				// Check for newly ready tasks
				for _, taskID := range executionOrder {
					statusMu.Lock()
					alreadyProcessed := taskStatus[taskID]
					alreadySubmitted := taskSubmitted[taskID]
					statusMu.Unlock()

					if !alreadyProcessed && !alreadySubmitted {
						checkAndSubmit(taskID)
					}
				}
			}
		}
	}()

	// Wait for all tasks to complete
	wg.Wait()
	
	// Signal monitoring goroutine to stop
	e.cancel()
	
	// Wait for monitoring goroutine to finish
	<-monitorDone

	// Check for errors
	if len(executionErrors) > 0 {
		return fmt.Errorf("some tasks failed: %d errors", len(executionErrors))
	}

	return nil
}

// Stop cancels execution
func (e *DependencyExecutor) Stop() {
	e.cancel()
}
