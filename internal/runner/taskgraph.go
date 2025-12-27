package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// TaskNode represents a task in the dependency graph
type TaskNode struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Dependencies []string `json:"dependencies,omitempty"` // IDs of dependencies
	Status        TaskStatus `json:"status"`
	Error       error     `json:"-"`
	mu          sync.RWMutex
}

// TaskStatus represents the execution status of a task
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskReady      TaskStatus = "ready"      // All dependencies satisfied
	TaskInProgress TaskStatus = "in_progress"
	TaskDone       TaskStatus = "done"
	TaskFailed     TaskStatus = "failed"
)

// TaskGraph represents a directed acyclic graph of tasks
type TaskGraph struct {
	Nodes map[string]*TaskNode `json:"nodes"`
	mu    sync.RWMutex
}

// NewTaskGraph creates a new empty task graph
func NewTaskGraph() *TaskGraph {
	return &TaskGraph{
		Nodes: make(map[string]*TaskNode),
	}
}

// AddNode adds a task node to the graph
func (g *TaskGraph) AddNode(id, name string, dependencies []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.Nodes == nil {
		g.Nodes = make(map[string]*TaskNode)
	}
	
	g.Nodes[id] = &TaskNode{
		ID:           id,
		Name:         name,
		Dependencies: dependencies,
		Status:       TaskPending,
	}
}

// LoadFromFeatureList loads tasks from feature_list.json with dependency support
func (g *TaskGraph) LoadFromFeatureList(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read feature_list.json: %w", err)
	}

	var features []struct {
		Category    string   `json:"category"`
		Description string   `json:"description"`
		Steps       []string `json:"steps"`
		Passes      bool     `json:"passes"`
		Dependencies []string `json:"dependencies,omitempty"` // Optional dependency field
	}

	if err := json.Unmarshal(data, &features); err != nil {
		return fmt.Errorf("failed to parse feature_list.json: %w", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	
	if g.Nodes == nil {
		g.Nodes = make(map[string]*TaskNode)
	}

	// Create task nodes from features
	for i, feature := range features {
		taskID := fmt.Sprintf("task-%d", i)
		deps := feature.Dependencies
		if deps == nil {
			deps = []string{}
		}
		
		g.Nodes[taskID] = &TaskNode{
			ID:           taskID,
			Name:         feature.Description,
			Dependencies: deps,
			Status:       TaskPending,
		}
	}

	return nil
}

// DetectCycles detects circular dependencies using DFS
func (g *TaskGraph) DetectCycles() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	cycle := []string{}

	var dfs func(nodeID string, path []string) bool
	dfs = func(nodeID string, path []string) bool {
		node, exists := g.Nodes[nodeID]
		if !exists {
			return false
		}

		visited[nodeID] = true
		recStack[nodeID] = true
		currentPath := append(path, nodeID)

		for _, depID := range node.Dependencies {
			if !visited[depID] {
				if dfs(depID, currentPath) {
					return true
				}
			} else if recStack[depID] {
				// Found a cycle
				cycleStart := -1
				for i, id := range currentPath {
					if id == depID {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle = append(currentPath[cycleStart:], depID)
					return true
				}
			}
		}

		recStack[nodeID] = false
		return false
	}

	for nodeID := range g.Nodes {
		if !visited[nodeID] {
			if dfs(nodeID, []string{}) {
				return cycle, fmt.Errorf("circular dependency detected: %v", cycle)
			}
		}
	}

	return nil, nil
}

// TopologicalSort returns tasks in dependency order (Kahn's algorithm)
func (g *TaskGraph) TopologicalSort() ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// Check for cycles first
	if cycle, err := g.DetectCycles(); err != nil {
		return nil, err
	} else if cycle != nil {
		return nil, fmt.Errorf("circular dependency detected: %v", cycle)
	}

	// Calculate in-degree for each node
	inDegree := make(map[string]int)
	for nodeID := range g.Nodes {
		inDegree[nodeID] = 0
	}

	for nodeID, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; exists {
				inDegree[nodeID]++
			}
		}
	}

	// Find all nodes with no dependencies (in-degree = 0)
	queue := []string{}
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	result := []string{}

	// Process nodes in topological order
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree of dependent nodes
		for nodeID, node := range g.Nodes {
			for _, depID := range node.Dependencies {
				if depID == current {
					inDegree[nodeID]--
					if inDegree[nodeID] == 0 {
						queue = append(queue, nodeID)
					}
				}
			}
		}
	}

	// Check if all nodes were processed (handles disconnected components)
	if len(result) < len(g.Nodes) {
		return nil, fmt.Errorf("graph has disconnected components or unresolved dependencies")
	}

	return result, nil
}

// GetReadyTasks returns all tasks that are ready to execute (all dependencies satisfied)
func (g *TaskGraph) GetReadyTasks() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ready := []string{}

	for nodeID, node := range g.Nodes {
		if node.Status != TaskPending && node.Status != TaskReady {
			continue
		}

		allDepsDone := true
		for _, depID := range node.Dependencies {
			dep, exists := g.Nodes[depID]
			if !exists {
				allDepsDone = false
				break
			}
			if dep.Status != TaskDone {
				allDepsDone = false
				break
			}
		}

		if allDepsDone {
			ready = append(ready, nodeID)
		}
	}

	return ready
}

// MarkTaskStatus updates the status of a task
func (g *TaskGraph) MarkTaskStatus(taskID string, status TaskStatus, err error) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.Nodes[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	node.mu.Lock()
	defer node.mu.Unlock()
	
	node.Status = status
	node.Error = err

	return nil
}

// GetTaskStatus returns the status of a task
func (g *TaskGraph) GetTaskStatus(taskID string) (TaskStatus, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.Nodes[taskID]
	if !exists {
		return TaskPending, fmt.Errorf("task %s not found", taskID)
	}

	node.mu.RLock()
	defer node.mu.RUnlock()
	
	return node.Status, nil
}

// GetTask returns a task node by ID
func (g *TaskGraph) GetTask(taskID string) (*TaskNode, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, exists := g.Nodes[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	return node, nil
}

// AllTasksDone checks if all tasks are completed (done or failed)
func (g *TaskGraph) AllTasksDone() bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, node := range g.Nodes {
		if node.Status != TaskDone && node.Status != TaskFailed {
			return false
		}
	}
	return true
}

// GetTaskSummary returns a summary of task statuses
func (g *TaskGraph) GetTaskSummary() map[TaskStatus]int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	summary := make(map[TaskStatus]int)
	for _, node := range g.Nodes {
		summary[node.Status]++
	}
	return summary
}
