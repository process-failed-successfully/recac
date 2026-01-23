package runner

import (
	"context"
	"fmt"
	"os"
	"recac/internal/db"
	"recac/internal/docker"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockOrchestratorDocker implements DockerClient
type MockOrchestratorDocker struct {
	mock.Mock
}

func (m *MockOrchestratorDocker) CheckDaemon(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockOrchestratorDocker) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, env []string, user string) (string, error) {
	args := m.Called(ctx, imageRef, workspace, extraBinds, env, user)
	return args.String(0), args.Error(1)
}

func (m *MockOrchestratorDocker) StopContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockOrchestratorDocker) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	// Bypass expectation matching
	return "", nil
}

func (m *MockOrchestratorDocker) ExecAsUser(ctx context.Context, containerID string, user string, cmd []string) (string, error) {
	// Bypass expectation matching for this method to avoid slice matching issues
	return "", nil
}

func (m *MockOrchestratorDocker) ImageExists(ctx context.Context, tag string) (bool, error) {
	args := m.Called(ctx, tag)
	return args.Bool(0), args.Error(1)
}

func (m *MockOrchestratorDocker) ImageBuild(ctx context.Context, opts docker.ImageBuildOptions) (string, error) {
	args := m.Called(ctx, opts)
	return args.String(0), args.Error(1)
}

func (m *MockOrchestratorDocker) PullImage(ctx context.Context, imageRef string) error {
	args := m.Called(ctx, imageRef)
	return args.Error(0)
}

// MockOrchestratorAgent implements agent.Agent
type MockOrchestratorAgent struct {
	mock.Mock
}

func (m *MockOrchestratorAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockOrchestratorAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestOrchestrator_MaxAgents(t *testing.T) {
	o := &Orchestrator{
		MaxAgents: 5,
	}

	assert.Equal(t, 5, o.GetMaxAgents())

	o.SetMaxAgents(10)
	assert.Equal(t, 10, o.GetMaxAgents())

	// Concurrency test
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o.SetMaxAgents(20)
			_ = o.GetMaxAgents()
		}()
	}
	wg.Wait()
}

func TestOrchestrator_HasFailures(t *testing.T) {
	o := &Orchestrator{
		Graph: NewTaskGraph(),
	}

	// Initial state: no tasks
	assert.False(t, o.hasFailures())

	// Add a success task
	o.Graph.AddNode("task1", "feature1", nil)
	o.Graph.MarkTaskStatus("task1", TaskDone, nil)
	assert.False(t, o.hasFailures())

	// Add a failed task
	o.Graph.AddNode("task2", "feature2", nil)
	o.Graph.MarkTaskStatus("task2", TaskFailed, fmt.Errorf("failed"))
	assert.True(t, o.hasFailures())
}

func TestOrchestrator_ExecuteTask(t *testing.T) {
	// Setup mocks
	mockDocker := new(MockOrchestratorDocker)
	mockAgent := new(MockOrchestratorAgent)

	// Mock basic interactions for Session start
	mockDocker.On("CheckDaemon", mock.Anything).Return(nil)
	mockDocker.On("ImageExists", mock.Anything, mock.Anything).Return(true, nil)
	mockDocker.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-id", nil)
	mockDocker.On("StopContainer", mock.Anything, "container-id").Return(nil)

	// Mock Agent interaction for the task
	// The session will ask the agent. We return a response that finishes the task.
	// For example, "COMPLETED" or just no commands + QA pass logic.
	// To keep it simple, we make the agent say "I am done." and ensure the session finishes.
	// Session loop finishes when:
	// 1. MaxIterations reached
	// 2. COMPLETED signal
	// 3. QA Agent approves

	// We'll set MaxIterations to 1 so it runs once and stops.
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("I am done.", nil)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return("I am done.", nil)

	// We also need a mock DB store
	tmpDir := t.TempDir()
	dbPath := fmt.Sprintf("%s/recac.db", tmpDir)
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Create app_spec.txt
	err = os.WriteFile(fmt.Sprintf("%s/app_spec.txt", tmpDir), []byte("Spec"), 0644)
	require.NoError(t, err)

	o := &Orchestrator{
		Graph:             NewTaskGraph(),
		DB:                store,
		Docker:            mockDocker,
		Agent:             mockAgent,
		Project:           "test-project",
		Workspace:         tmpDir,
		TaskMaxIterations: 1, // Run once
		TaskMaxRetries:    0,
	}

	// Add task to graph
	taskID := "task-1"
	o.Graph.AddNode(taskID, "feature-1", nil)
	o.Graph.MarkTaskStatus(taskID, TaskInProgress, nil)

	node, _ := o.Graph.GetTask(taskID)

	ctx := context.Background()
	err = o.ExecuteTask(ctx, taskID, node)

	// Since we mocked minimal agent interaction and set MaxIterations=1,
	// The session run loop should finish.
	// Whether it marks task as Done or Failed depends on the session outcome.
	// If session returns nil error (max iterations reached w/o critical error), ExecuteTask marks it Done.
	// Wait, if MaxIterations reached, RunLoop returns nil?
	// Session.RunLoop returns nil if it completes cleanly.
	// If it hits max iterations without completion signal, it returns ErrMaxIterations.
	// Since our mock agent just says "I am done" without setting COMPLETED signal,
	// RunLoop will eventually fail with max iterations.

	// We expect an error here because we didn't force a success signal
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "maximum iterations reached")

	status, _ := o.Graph.GetTaskStatus(taskID)
	assert.Equal(t, TaskFailed, status)
}
