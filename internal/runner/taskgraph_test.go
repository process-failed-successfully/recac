package runner

import (
	"reflect"
	"testing"
)

func TestTaskGraph_AddNode(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("1", "Task 1", nil)

	if len(g.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(g.Nodes))
	}
	if g.Nodes["1"].Name != "Task 1" {
		t.Errorf("Expected name 'Task 1', got '%s'", g.Nodes["1"].Name)
	}
}

func TestTaskGraph_DetectCycles_NoCycle(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("1", "Task 1", []string{"2"})
	g.AddNode("2", "Task 2", []string{"3"})
	g.AddNode("3", "Task 3", nil)

	cycle, err := g.DetectCycles()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if cycle != nil {
		t.Errorf("Expected no cycle, got %v", cycle)
	}
}

func TestTaskGraph_DetectCycles_WithCycle(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("1", "Task 1", []string{"2"})
	g.AddNode("2", "Task 2", []string{"3"})
	g.AddNode("3", "Task 3", []string{"1"}) // Cycle: 1->2->3->1

	cycle, err := g.DetectCycles()
	if err == nil {
		t.Error("Expected error for cycle, got nil")
	}
	if cycle == nil {
		t.Error("Expected cycle path, got nil")
	}
}

func TestTaskGraph_TopologicalSort(t *testing.T) {
	g := NewTaskGraph()
	// 1 -> 2 -> 3
	g.AddNode("1", "Task 1", nil)
	g.AddNode("2", "Task 2", []string{"1"})
	g.AddNode("3", "Task 3", []string{"2"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected := []string{"1", "2", "3"}
	if !reflect.DeepEqual(order, expected) {
		t.Errorf("Expected order %v, got %v", expected, order)
	}
}

func TestTaskGraph_TopologicalSort_Disconnected(t *testing.T) {
	g := NewTaskGraph()
	// 1->2, 3->4
	g.AddNode("1", "Task 1", nil)
	g.AddNode("2", "Task 2", []string{"1"})
	g.AddNode("3", "Task 3", nil)
	g.AddNode("4", "Task 4", []string{"3"})

	order, err := g.TopologicalSort()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should contain all nodes
	if len(order) != 4 {
		t.Errorf("Expected 4 nodes, got %d", len(order))
	}
}

func TestTaskGraph_GetReadyTasks(t *testing.T) {
	g := NewTaskGraph()
	// 1 (done) -> 2 (pending)
	// 3 (pending) -> 4 (pending)
	
	g.AddNode("1", "Task 1", nil)
	g.AddNode("2", "Task 2", []string{"1"})
	g.AddNode("3", "Task 3", nil)
	g.AddNode("4", "Task 4", []string{"3"})

	// Initially, 1 and 3 should be ready
	ready := g.GetReadyTasks()
	
	// Since using map, order is random, check for containment
	if len(ready) != 2 {
		t.Errorf("Expected 2 ready tasks, got %d", len(ready))
	}

	g.MarkTaskStatus("1", TaskDone, nil)
	
	// Now 2 and 3 should be ready (1 is done, so 2 becomes ready. 3 is still ready)
	ready = g.GetReadyTasks()
	
	// 1 is done, so it's not "ready" (pending/ready status filter in GetReadyTasks might exclude done tasks? let's check implementation)
	// Implementation: if node.Status != TaskPending && node.Status != TaskReady { continue }
	// So done tasks are excluded.
	
	// 2 depends on 1 (Done). So 2 is ready.
	// 3 depends on nothing. So 3 is ready.
	// 4 depends on 3 (Pending). So 4 is not ready.
	
	expected := map[string]bool{"2": true, "3": true}
	found := 0
	for _, id := range ready {
		if expected[id] {
			found++
		}
	}
	if found != 2 {
		t.Errorf("Expected tasks 2 and 3 to be ready, got %v", ready)
	}
}

func TestTaskGraph_TaskStatus(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("1", "Task 1", nil)

	status, err := g.GetTaskStatus("1")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if status != TaskPending {
		t.Errorf("Expected status Pending, got %s", status)
	}

	g.MarkTaskStatus("1", TaskInProgress, nil)
	status, _ = g.GetTaskStatus("1")
	if status != TaskInProgress {
		t.Errorf("Expected status InProgress, got %s", status)
	}
	
	// Test unknown task
	_, err = g.GetTaskStatus("999")
	if err == nil {
		t.Error("Expected error for unknown task, got nil")
	}
}

func TestTaskGraph_AllTasksDone(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("1", "Task 1", nil)
	
	if g.AllTasksDone() {
		t.Error("Expected false when task is pending")
	}
	
	g.MarkTaskStatus("1", TaskDone, nil)
	if !g.AllTasksDone() {
		t.Error("Expected true when all tasks done")
	}
	
	g.MarkTaskStatus("1", TaskFailed, nil)
	if !g.AllTasksDone() {
		t.Error("Expected true when task failed (completed)")
	}
}

func TestTaskGraph_GetTaskSummary(t *testing.T) {
	g := NewTaskGraph()
	g.AddNode("1", "Task 1", nil)
	g.AddNode("2", "Task 2", nil)
	
	g.MarkTaskStatus("1", TaskDone, nil)
	g.MarkTaskStatus("2", TaskPending, nil)
	
	summary := g.GetTaskSummary()
	if summary[TaskDone] != 1 {
		t.Errorf("Expected 1 done task, got %d", summary[TaskDone])
	}
	if summary[TaskPending] != 1 {
		t.Errorf("Expected 1 pending task, got %d", summary[TaskPending])
	}
}

func TestTaskGraph_LoadFromFeatureList(t *testing.T) {
    // We need a temp file for this
    // Skipping for now or mocking os.ReadFile if we could, 
    // but better to rely on integration tests or mocking the file system.
    // For unit test, we can try to write a temp file.
}
