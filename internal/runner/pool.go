package runner

import (
	"context"
	"fmt"
	"recac/internal/notify"
	"sync"
)

// Task represents a unit of work.
type Task func(workerID int) error

// WorkerPool manages a pool of worker goroutines.
type WorkerPool struct {
	NumWorkers int
	Tasks      chan Task
	notifier   notify.Notifier
	mu         sync.RWMutex
	wg         sync.WaitGroup
}

// NewWorkerPool creates a new worker pool.
func NewWorkerPool(numWorkers int) *WorkerPool {
	return &WorkerPool{
		NumWorkers: numWorkers,
		Tasks:      make(chan Task),
	}
}

// SetNotifier sets the notifier for the pool in a thread-safe way.
func (p *WorkerPool) SetNotifier(n notify.Notifier) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.notifier = n
}

// GetNotifier returns the current notifier in a thread-safe way.
func (p *WorkerPool) GetNotifier() notify.Notifier {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.notifier
}

// Start launches the worker goroutines.
func (p *WorkerPool) Start() {
	fmt.Printf("Starting worker pool with %d workers\n", p.NumWorkers)
	for i := 0; i < p.NumWorkers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()
	fmt.Printf("Worker %d started\n", id)
	for task := range p.Tasks {
		if err := task(id); err != nil {
			fmt.Printf("Worker %d error: %v\n", id, err)
		} else {
			n := p.GetNotifier()
			if n != nil {
				_ = n.Notify(context.Background(), fmt.Sprintf("Worker %d completed a task", id))
			}
		}
	}
	fmt.Printf("Worker %d stopped\n", id)
}

// Submit adds a task to the pool.
func (p *WorkerPool) Submit(t Task) {
	p.Tasks <- t
}

// Stop closes the task channel and waits for workers to finish.
func (p *WorkerPool) Stop() {
	close(p.Tasks)
	p.wg.Wait()
	fmt.Println("Worker pool stopped")
}
