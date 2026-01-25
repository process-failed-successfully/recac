package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type PruneMockDockerClient struct {
	RemovedContainers []string
}

func (m *PruneMockDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	m.RemovedContainers = append(m.RemovedContainers, containerID)
	return nil
}

func (m *PruneMockDockerClient) Close() error {
	return nil
}

func TestPruneCommand_Cleanup(t *testing.T) {
	// Setup Session Manager
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Setup Mock Docker Client
	mockDocker := &PruneMockDockerClient{}
	origDockerFactory := dockerClientFactory
	dockerClientFactory = func(project string) (IDockerClient, error) {
		return mockDocker, nil
	}
	defer func() { dockerClientFactory = origDockerFactory }()

	// Create a temporary workspace directory that SHOULD be deleted
	// We use "recac-agent-" prefix to satisfy the safety check
	tempWorkspace, err := os.MkdirTemp("", "recac-agent-test-prune-")
	require.NoError(t, err)

	// Create a file in it to ensure it's not empty
	err = os.WriteFile(filepath.Join(tempWorkspace, "dummy"), []byte("data"), 0644)
	require.NoError(t, err)

	// Create a session with Workspace and ContainerID
	session := &runner.SessionState{
		Name:        "prune-test-cleanup",
		Status:      "completed",
		StartTime:   time.Now().Add(-2 * time.Hour),
		Workspace:   tempWorkspace,
		ContainerID: "test-container-123",
		PID:         0,
	}

	// Save session
	logPath := filepath.Join(sm.SessionsDir(), session.Name+".log")
	session.LogFile = logPath
	require.NoError(t, os.WriteFile(logPath, []byte("log"), 0644))
	require.NoError(t, sm.SaveSession(session))

	// Run Prune
	output, err := executeCommand(rootCmd, "prune", "--all") // Use --all to ignore filters just in case
	require.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "Pruned session: prune-test-cleanup")

	// Verify Container Removal
	assert.Contains(t, mockDocker.RemovedContainers, "test-container-123")

	// Verify Workspace Removal
	_, err = os.Stat(tempWorkspace)
	assert.True(t, os.IsNotExist(err), "Workspace should have been deleted")

	// Verify Session Files Removal
	_, err = sm.LoadSession("prune-test-cleanup")
	assert.ErrorIs(t, err, os.ErrNotExist)
}
