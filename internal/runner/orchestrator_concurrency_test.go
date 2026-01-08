package runner

import (
	"context"
	"os"
	"recac/internal/agent"
	"recac/internal/db"
	"testing"
	"time"
)

func TestOrchestrator_ConcurrencyLimit(t *testing.T) {
	// Setup
	mockDB := &MockDBStoreForOrchestrator{
		Features: `{"features": [
			{"id": "task-1", "status": "pending", "passes": false, "dependencies": {}},
			{"id": "task-2", "status": "pending", "passes": false, "dependencies": {}},
			{"id": "task-3", "status": "pending", "passes": false, "dependencies": {"depends_on_ids": ["task-1"]}}
		]}`, // Tasks 1 and 2 are ready. Task 3 is blocked. Count = 2.
	}
	mockDocker := &MockDockerForOrchestrator{}
	mockAgent := &agent.MockAgent{}

	// Setup workspace
	workspace, err := os.MkdirTemp("", "orch_concurrency_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workspace)

	// Initialize Orchestrator with MaxAgents = 10
	orch := NewOrchestrator(mockDB, mockDocker, workspace, "test-image", mockAgent, "test-project", "gemini", "gemini-pro", 10, "")

	// Since Run() blocks and is complex, we'll spy on the side effect we care about.
	// We want to verify that orch.MaxAgents becomes 2 and orch.Pool.NumWorkers becomes 2.
	// We can't easily run Run() completely without mocking a lot, so let's use a trick:
	// We'll trust that our logic uses GetReadyTasks and clamps.

	// Actually, to make this testable without a massive mock setup for the Run loop,
	// we should probably extract the adjustment logic into a helper method or
	// construct the test such that we fail fast in the loop or run it in a goroutine
	// and check the state after a short delay.

	// Let's use the goroutine approach with a cancel context.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Intercept the execution to verify state
	// We can't easily intercept inside Run without modifying code primarily for testing.
	// However, since we are allowed to modify code, maybe we just verify by running it
	// and checking the struct state afterwards?
	// But Run blocks until completion or error.

	// Mock Graph loading by pre-loading it since refreshGraph uses DB
	// NewOrchestrator initializes Graph.
	// refreshGraph calls DB.GetFeatures then LoadFromFeatureList.
	// We can manually load the graph to ensure state is correct before Run logic triggers.
	// BUT Run calls refreshGraph() first thing.

	// Run the orchestrator
	go func() {
		_ = orch.Run(ctx)
	}()

	// Wait a bit for initialization (Run calls refreshGraph -> logic -> Start)
	time.Sleep(50 * time.Millisecond)

	// Assertions
	expected := 2
	if orch.GetMaxAgents() != expected {
		t.Errorf("Expected MaxAgents to be adjusted to %d, got %d", expected, orch.GetMaxAgents())
	}
	if orch.Pool.GetNumWorkers() != expected {
		t.Errorf("Expected Pool.NumWorkers to be adjusted to %d, got %d", expected, orch.Pool.GetNumWorkers())
	}
}

// Minimal Mocks needed for this specific test
type MockDBStoreForOrchestrator struct {
	db.Store // Embed interface to skip implementing everything
	Features string
}

func (m *MockDBStoreForOrchestrator) GetFeatures(projectID string) (string, error) {
	return m.Features, nil
}
func (m *MockDBStoreForOrchestrator) GetSignal(projectID, name string) (string, error) {
	return "", nil
}
func (m *MockDBStoreForOrchestrator) SetSignal(projectID, name, value string) error { return nil }
func (m *MockDBStoreForOrchestrator) GetActiveLocks(projectID string) ([]db.Lock, error) {
	return nil, nil
}
func (m *MockDBStoreForOrchestrator) Close() error { return nil }
func (m *MockDBStoreForOrchestrator) SaveObservation(projectID, role, content string) error {
	return nil
}
func (m *MockDBStoreForOrchestrator) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	return nil, nil
}
func (m *MockDBStoreForOrchestrator) DeleteSignal(projectID, name string) error       { return nil }
func (m *MockDBStoreForOrchestrator) SaveFeatures(projectID, features string) error   { return nil }
func (m *MockDBStoreForOrchestrator) ReleaseAllLocks(projectID, agentID string) error { return nil }
func (m *MockDBStoreForOrchestrator) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	return true, nil
}
func (m *MockDBStoreForOrchestrator) ReleaseLock(projectID, path, agentID string) error { return nil }
func (m *MockDBStoreForOrchestrator) Cleanup() error                                    { return nil }
func (m *MockDBStoreForOrchestrator) UpdateFeatureStatus(projectID, id, status string, passes bool) error {
	return nil
}

type MockDockerForOrchestrator struct {
	DockerClient // Embed
}

func (m *MockDockerForOrchestrator) CheckDaemon(ctx context.Context) error { return nil }
func (m *MockDockerForOrchestrator) RunContainer(ctx context.Context, image, workspace string, extraBinds []string, env []string, user string) (string, error) {
	return "mock-container-id", nil
}
func (m *MockDockerForOrchestrator) StopContainer(ctx context.Context, id string) error { return nil }
func (m *MockDockerForOrchestrator) ImageExists(ctx context.Context, image string) (bool, error) {
	return true, nil
}
