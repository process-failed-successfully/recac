package runner

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/db"
	"sync"
)

// TaskNode represents a task in the dependency graph
type TaskNode struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Priority            string     `json:"priority"`
	Dependencies        []string   `json:"dependencies,omitempty"` // IDs of dependencies
	ExclusiveWritePaths []string   `json:"exclusive_write_paths,omitempty"`
	ReadOnlyPaths       []string   `json:"read_only_paths,omitempty"`
	Status              TaskStatus `json:"status"`
	RetryCount          int        `json:"retry_count"`
	Error               error      `json:"-"`
	mu                  sync.RWMutex
}

// TaskStatus represents the execution status of a task
type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskReady      TaskStatus = "ready" // All dependencies satisfied
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

	var featureList db.FeatureList
	if err := json.Unmarshal(data, &featureList); err != nil {
		return fmt.Errorf("failed to parse feature_list.json: %w", err)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.Nodes == nil {
		g.Nodes = make(map[string]*TaskNode)
	}

	// Create or update task nodes from features
	for _, feature := range featureList.Features {
		taskID := feature.ID
		if taskID == "" {
			taskID = fmt.Sprintf("task-%s", feature.Category)
		}

		deps := feature.Dependencies.DependsOnIDs
		if deps == nil {
			deps = []string{}
		}

		// Map generic status to TaskStatus
		var newStatus TaskStatus
		if feature.Passes || feature.Status == "done" || feature.Status == "implemented" {
			newStatus = TaskDone
		} else {
			switch feature.Status {
			case "in_progress":
				newStatus = TaskInProgress
			case "failed":
				newStatus = TaskFailed
			default:
				newStatus = TaskPending
			}
		}

		// Update existing node or create new one
		if existing, ok := g.Nodes[taskID]; ok {
			existing.mu.Lock()
			// NEVER downgrade InProgress or Done to Pending/Ready from external sync
			// unless we explicitly want a force-reset (not handled here).
			// This prevents race conditions where the orchestrator resets a task
			// that is currently being worked on by an agent.
			if (existing.Status == TaskInProgress || existing.Status == TaskDone) && (newStatus == TaskPending || newStatus == TaskReady) {
				// Keep existing status
			} else {
				existing.Status = newStatus
			}
			// Update metadata that might have changed
			existing.Name = feature.Description
			existing.Priority = feature.Priority
			existing.Dependencies = deps
			existing.ExclusiveWritePaths = feature.Dependencies.ExclusiveWritePaths
			existing.ReadOnlyPaths = feature.Dependencies.ReadOnlyPaths
			existing.mu.Unlock()
		} else {
			g.Nodes[taskID] = &TaskNode{
				ID:                  taskID,
				Name:                feature.Description,
				Priority:            feature.Priority,
				Dependencies:        deps,
				ExclusiveWritePaths: feature.Dependencies.ExclusiveWritePaths,
				ReadOnlyPaths:       feature.Dependencies.ReadOnlyPaths,
				Status:              newStatus,
			}
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

	// Build adjacency list (forward dependencies: node -> things that depend on it)
	// and calculate in-degrees
	dependents := make(map[string][]string)
	inDegree := make(map[string]int)
	for nodeID := range g.Nodes {
		inDegree[nodeID] = 0
	}

	for nodeID, node := range g.Nodes {
		for _, depID := range node.Dependencies {
			if _, exists := g.Nodes[depID]; exists {
				dependents[depID] = append(dependents[depID], nodeID)
				inDegree[nodeID]++
			}
		}
	}

	// Find all nodes with no dependencies (in-degree = 0)
	queue := make([]string, 0, len(g.Nodes))
	for nodeID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, nodeID)
		}
	}

	result := make([]string, 0, len(g.Nodes))

	// Process nodes in topological order
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree of dependent nodes
		for _, dependentID := range dependents[current] {
			inDegree[dependentID]--
			if inDegree[dependentID] == 0 {
				queue = append(queue, dependentID)
			}
		}
	}

	// Check if all nodes were processed
	if len(result) < len(g.Nodes) {
		return nil, fmt.Errorf("graph has circular dependencies or unresolved dependencies")
	}

	return result, nil
}

// GetReadyTasks returns all tasks that are ready to execute (all dependencies satisfied)
func (g *TaskGraph) GetReadyTasks() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ready := []string{}

	for nodeID, node := range g.Nodes {
		node.mu.RLock()
		status := node.Status
		deps := node.Dependencies
		node.mu.RUnlock()

		if status != TaskPending && status != TaskReady {
			continue
		}

		allDepsDone := true
		for _, depID := range deps {
			dep, exists := g.Nodes[depID]
			if !exists {
				allDepsDone = false
				break
			}
			dep.mu.RLock()
			depStatus := dep.Status
			dep.mu.RUnlock()
			if depStatus != TaskDone {
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
