package main

import (
	"context"
	"fmt"
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
	RemoveError       error
}

func (m *PruneMockDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if m.RemoveError != nil {
		return m.RemoveError
	}
	m.RemovedContainers = append(m.RemovedContainers, containerID)
	return nil
}

func (m *PruneMockDockerClient) Close() error {
	return nil
}

func TestPruneCommand_Extended(t *testing.T) {
	// Setup Session Manager
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// Capture original factory
	origDockerFactory := dockerClientFactory
	defer func() { dockerClientFactory = origDockerFactory }()

	t.Run("Cleanup Success", func(t *testing.T) {
		mockDocker := &PruneMockDockerClient{}
		dockerClientFactory = func(project string) (IDockerClient, error) {
			return mockDocker, nil
		}

		tempWorkspace, err := os.MkdirTemp("", "recac-agent-test-prune-")
		require.NoError(t, err)

		err = os.WriteFile(filepath.Join(tempWorkspace, "dummy"), []byte("data"), 0644)
		require.NoError(t, err)

		createSession(t, sm, "prune-success", tempWorkspace, "container-1")

		output, err := executeCommand(rootCmd, "prune", "--all")
		require.NoError(t, err)

		assert.Contains(t, output, "Pruned session: prune-success")
		assert.Contains(t, mockDocker.RemovedContainers, "container-1")
		_, err = os.Stat(tempWorkspace)
		assert.True(t, os.IsNotExist(err), "Workspace should have been deleted")
	})

	t.Run("Docker Remove Error", func(t *testing.T) {
		mockDocker := &PruneMockDockerClient{
			RemoveError: fmt.Errorf("daemon failed"),
		}
		dockerClientFactory = func(project string) (IDockerClient, error) {
			return mockDocker, nil
		}

		createSession(t, sm, "prune-docker-error", "", "container-fail")

		output, err := executeCommand(rootCmd, "prune", "--all")
		require.NoError(t, err)

		// It should still prune the session
		assert.Contains(t, output, "Pruned session: prune-docker-error")
		// It should log the error
		assert.Contains(t, output, "Failed to remove container container-fail: daemon failed")
	})

	t.Run("Docker Remove Ignored Error", func(t *testing.T) {
		mockDocker := &PruneMockDockerClient{
			RemoveError: fmt.Errorf("No such container: container-gone"),
		}
		dockerClientFactory = func(project string) (IDockerClient, error) {
			return mockDocker, nil
		}

		createSession(t, sm, "prune-docker-ignored", "", "container-gone")

		output, err := executeCommand(rootCmd, "prune", "--all")
		require.NoError(t, err)

		// It should still prune
		assert.Contains(t, output, "Pruned session: prune-docker-ignored")
		// It should NOT log the error
		assert.NotContains(t, output, "Failed to remove container")
	})
}

func createSession(t *testing.T, sm ISessionManager, name, workspace, containerID string) *runner.SessionState {
	session := &runner.SessionState{
		Name:        name,
		Status:      "completed",
		StartTime:   time.Now().Add(-1 * time.Hour),
		Workspace:   workspace,
		ContainerID: containerID,
	}
	logPath := filepath.Join(sm.SessionsDir(), name+".log")
	session.LogFile = logPath
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		require.NoError(t, os.WriteFile(logPath, []byte("log"), 0644))
	}
	require.NoError(t, sm.SaveSession(session))
	return session
}
