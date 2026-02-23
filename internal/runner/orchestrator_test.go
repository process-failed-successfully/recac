package runner

import (
	"context"
	"encoding/json"
	"errors"
	"recac/internal/agent"
	"recac/internal/db"
	"sync"
	"testing"
	"time"
)

// TestMockDB implements db.Store interface for testing Orchestrator
type TestMockDB struct {
	mu          sync.Mutex
	Features    db.FeatureList
	Signals     map[string]string
	Locks       map[string]string // path -> agentID
	Specs       map[string]string
	Saved       []string // Track saved features for verification
}

func NewTestMockDB() *TestMockDB {
	return &TestMockDB{
		Signals:  make(map[string]string),
		Locks:    make(map[string]string),
		Specs:    make(map[string]string),
		Features: db.FeatureList{Features: []db.Feature{}},
	}
}

func (m *TestMockDB) Close() error { return nil }
func (m *TestMockDB) SaveObservation(projectID, agentID, content string) error { return nil }
func (m *TestMockDB) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	return nil, nil
}

func (m *TestMockDB) SaveFeatures(projectID, features string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Saved = append(m.Saved, features)
	var fl db.FeatureList
	if err := json.Unmarshal([]byte(features), &fl); err != nil {
		return err
	}
	m.Features = fl
	return nil
}

func (m *TestMockDB) GetFeatures(projectID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := json.Marshal(m.Features)
	return string(data), err
}

func (m *TestMockDB) SetSignal(projectID, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Signals[key] = value
	return nil
}

func (m *TestMockDB) GetSignal(projectID, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Signals[key], nil
}

func (m *TestMockDB) DeleteSignal(projectID, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Signals, key)
	return nil
}

func (m *TestMockDB) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Locks == nil {
		m.Locks = make(map[string]string)
	}
	holder, exists := m.Locks[path]
	if !exists {
		m.Locks[path] = agentID
		return true, nil
	}
	if holder == agentID {
		return true, nil
	}
	return false, nil
}

func (m *TestMockDB) ReleaseLock(projectID, path, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Locks[path] == agentID {
		delete(m.Locks, path)
	}
	return nil
}

func (m *TestMockDB) GetActiveLocks(projectID string) ([]db.Lock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var locks []db.Lock
	for path, agentID := range m.Locks {
		locks = append(locks, db.Lock{Path: path, AgentID: agentID})
	}
	return locks, nil
}

func (m *TestMockDB) ReleaseAllLocks(projectID, agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for path, holder := range m.Locks {
		if holder == agentID {
			delete(m.Locks, path)
		}
	}
	return nil
}

func (m *TestMockDB) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.Features.Features {
		if m.Features.Features[i].ID == id {
			m.Features.Features[i].Status = status
			m.Features.Features[i].Passes = passes
			return nil
		}
	}
	return errors.New("feature not found")
}

func (m *TestMockDB) SaveSpec(projectID string, spec string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Specs[projectID] = spec
	return nil
}

func (m *TestMockDB) GetSpec(projectID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Specs[projectID], nil
}

func (m *TestMockDB) Cleanup() error { return nil }

// TestMockAgent implements agent.Agent
type TestMockAgent struct {
	Response string
	Stream   bool
}

func (m *TestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *TestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.Stream && onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func (m *TestMockAgent) WithStateManager(sm *agent.StateManager) agent.Agent {
	return m
}

func TestOrchestrator_Run_Signals(t *testing.T) {
	dbStore := NewTestMockDB()
	dbStore.Features = db.FeatureList{
		Features: []db.Feature{
			{ID: "task-1", Status: "pending", Description: "Task 1"},
		},
	}

	mockDocker := &MockDockerClient{}
	mockAgent := &TestMockAgent{Response: "Done"}

	orch := NewOrchestrator(dbStore, mockDocker, "/tmp", "image", mockAgent, "proj", "prov", "model", 1, "")
	orch.TickInterval = 10 * time.Millisecond

	// Set signal to stop orchestration
	dbStore.SetSignal("proj", "QA_PASSED", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Run should exit immediately due to signal (after loading graph)
	err := orch.Run(ctx)
	if err != nil {
		t.Errorf("Run failed: %v", err)
	}
}

func TestOrchestrator_ExecuteTask_LockContention(t *testing.T) {
	dbStore := NewTestMockDB()
	dbStore.Locks["file.txt"] = "other-agent"

	node := &TaskNode{
		ID:                  "task-1",
		ExclusiveWritePaths: []string{"file.txt"},
		Status:              TaskPending,
		mu:                  sync.RWMutex{},
	}

	orch := &Orchestrator{
		DB:      dbStore,
		Project: "proj",
		Graph:   NewTaskGraph(),
	}
	// Manually inject node
	orch.Graph.Nodes["task-1"] = node

	err := orch.ExecuteTask(context.Background(), "task-1", node)
	if err == nil {
		t.Error("Expected error due to lock contention, got nil")
	}
	if err.Error() != "lock acquisition failed" {
		t.Errorf("Expected 'lock acquisition failed', got '%v'", err)
	}

	// Verify status marked as Pending (retry later)
	status, _ := orch.Graph.GetTaskStatus("task-1")
	if status != TaskPending {
		t.Errorf("Expected status TaskPending, got %s", status)
	}
}

func TestOrchestrator_ExecuteTask_RetryLogic(t *testing.T) {
	dbStore := NewTestMockDB()
	mockDocker := &MockDockerClient{}
	mockAgent := &TestMockAgent{Response: "Fail"}

	orch := NewOrchestrator(dbStore, mockDocker, "/tmp", "image", mockAgent, "proj", "prov", "model", 1, "")
	orch.TaskMaxRetries = 2
	orch.TaskMaxIterations = 1

	node := &TaskNode{
		ID:         "task-1",
		Status:     TaskInProgress,
		RetryCount: 0,
		mu:         sync.RWMutex{},
	}
	// Manually inject node
	orch.Graph.Nodes["task-1"] = node

	err := orch.ExecuteTask(context.Background(), "task-1", node)

	if err != nil {
		t.Errorf("Expected nil error (retry scheduled), got %v", err)
	}

	node.mu.Lock()
	if node.RetryCount != 1 {
		t.Errorf("Expected RetryCount 1, got %d", node.RetryCount)
	}
	if node.Status != TaskPending {
		t.Errorf("Expected status reset to Pending, got %s", node.Status)
	}
	node.mu.Unlock()
}

func TestOrchestrator_ExecuteTask_SessionStartFailure(t *testing.T) {
	dbStore := NewTestMockDB()
	mockDocker := &MockDockerClient{
		RunContainerFunc: func(ctx context.Context, image, workspace string, extraBinds, env, cmd []string, user string) (string, error) {
			return "", errors.New("docker failure")
		},
	}
	mockAgent := &TestMockAgent{Response: "Done"}

	orch := NewOrchestrator(dbStore, mockDocker, "/tmp", "image", mockAgent, "proj", "prov", "model", 1, "")

	node := &TaskNode{
		ID: "task-1", Status: TaskInProgress,
	}
	// Manually inject node
	orch.Graph.Nodes["task-1"] = node

	err := orch.ExecuteTask(context.Background(), "task-1", node)

	if err == nil {
		t.Error("Expected error due to docker failure, got nil")
	}

	status, _ := orch.Graph.GetTaskStatus("task-1")
	if status != TaskFailed {
		t.Errorf("Expected TaskFailed, got %s", status)
	}
}

func TestOrchestrator_CanAcquireImmediate(t *testing.T) {
	dbStore := NewTestMockDB()
	orch := &Orchestrator{DB: dbStore, Graph: NewTaskGraph(), Project: "proj"}

	// Case 1: DB Lock exists
	dbStore.Locks["file.txt"] = "other"
	if orch.canAcquireImmediate([]string{"file.txt"}) {
		t.Error("Expected false (DB locked)")
	}

	// Case 2: Graph Lock exists (InProgress task)
	dbStore.Locks = make(map[string]string)
	node := &TaskNode{ID: "t1", Status: TaskInProgress, ExclusiveWritePaths: []string{"file.txt"}}
	orch.Graph.Nodes["t1"] = node

	if orch.canAcquireImmediate([]string{"file.txt"}) {
		t.Error("Expected false (Graph locked)")
	}

	// Case 3: Free
	if !orch.canAcquireImmediate([]string{"other.txt"}) {
		t.Error("Expected true")
	}
}

func TestOrchestrator_Run_HighFailureRate(t *testing.T) {
	dbStore := NewTestMockDB()
	// 3 tasks, 2 failed -> 66% failure rate
	dbStore.Features = db.FeatureList{
		Features: []db.Feature{
			{ID: "task-1", Status: "failed"},
			{ID: "task-2", Status: "failed"},
			{ID: "task-3", Status: "pending"}, // Keeps running
		},
	}

	mockDocker := &MockDockerClient{}
	mockAgent := &TestMockAgent{Response: "Wait"}

	orch := NewOrchestrator(dbStore, mockDocker, "/tmp", "image", mockAgent, "proj", "prov", "model", 1, "")
	orch.TickInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go orch.Run(ctx)

	// Wait for loop to trigger logic
	time.Sleep(100 * time.Millisecond)

	// Check signal
	sig, _ := dbStore.GetSignal("proj", "TRIGGER_MANAGER")
	if sig != "true" {
		t.Errorf("Expected TRIGGER_MANAGER signal due to high failure rate, got %s", sig)
	}
}

func TestOrchestrator_DeadlockDetection(t *testing.T) {
	dbStore := NewTestMockDB()

	feat1 := db.Feature{ID: "task-A", Status: "pending", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"task-B"}}}
	feat2 := db.Feature{ID: "task-B", Status: "failed"}

	dbStore.Features = db.FeatureList{Features: []db.Feature{feat1, feat2}}

	mockDocker := &MockDockerClient{}
	mockAgent := &TestMockAgent{Response: "Done"}

	orch := NewOrchestrator(dbStore, mockDocker, "/tmp", "image", mockAgent, "proj", "prov", "model", 1, "")
	orch.TickInterval = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	go orch.Run(ctx)

	time.Sleep(100 * time.Millisecond)

	statusA, _ := orch.Graph.GetTaskStatus("task-A")
	if statusA != TaskFailed {
		t.Errorf("Expected task-A to be Failed (deadlock due to failed dependency), got %s", statusA)
	}
}
