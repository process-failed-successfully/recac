package orchestrator_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// IntegrationMockSpawner is a mock implementation of the Spawner interface
type IntegrationMockSpawner struct {
	mock.Mock
}

func (m *IntegrationMockSpawner) Spawn(ctx context.Context, item orchestrator.WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *IntegrationMockSpawner) Cleanup(ctx context.Context, item orchestrator.WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *IntegrationMockSpawner) Cancel(ctx context.Context, jobID string) error {
	args := m.Called(ctx, jobID)
	return args.Error(0)
}

func (m *IntegrationMockSpawner) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *IntegrationMockSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	args := m.Called(ctx, jobID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func TestOrchestrator_FileDirPoller_Integration(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "orchestrator-integration-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	poller, err := orchestrator.NewFileDirPoller(tmpDir)
	require.NoError(t, err)

	spawner := new(IntegrationMockSpawner)

	// Orchestrator with a short interval for testing
	orch := orchestrator.New(poller, spawner, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Mocks
	workItem := orchestrator.WorkItem{ID: "test-task", Summary: "Test Task"}
	spawner.On("Spawn", mock.Anything, workItem).Return(nil)

	// Create a work file
	taskData, err := json.Marshal(workItem)
	require.NoError(t, err)
	taskPath := filepath.Join(tmpDir, "task.json")
	err = os.WriteFile(taskPath, taskData, 0644)
	require.NoError(t, err)

	// Run the orchestrator in a separate goroutine
	go func() {
		if err := orch.Run(ctx, logger); err != nil {
			if ctx.Err() == nil { // Don't fail on context cancellation
				t.Errorf("Orchestrator.Run() failed: %v", err)
			}
		}
	}()

	// Wait for the orchestrator to process the work item
	time.Sleep(500 * time.Millisecond)

	// Assertions
	spawner.AssertCalled(t, "Spawn", mock.Anything, workItem)

	// Verify the file was moved
	_, err = os.Stat(taskPath)
	assert.True(t, os.IsNotExist(err), "The original task file should have been moved")
	processedPath := filepath.Join(tmpDir, "processed", "task.json")
	_, err = os.Stat(processedPath)
	assert.NoError(t, err, "The task file should exist in the processed directory")
}
