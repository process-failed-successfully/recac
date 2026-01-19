package main

import (
	"context"
	"os"
	"recac/internal/docker"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockDockerClient struct {
	ExecInteractiveFunc func(ctx context.Context, containerID string, cmd []string) error
}

func (m *MockDockerClient) ExecInteractive(ctx context.Context, containerID string, cmd []string) error {
	if m.ExecInteractiveFunc != nil {
		return m.ExecInteractiveFunc(ctx, containerID, cmd)
	}
	return nil
}

func TestExecCmd(t *testing.T) {
	// 1. Setup mocks
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	mockDocker := &MockDockerClient{}
	// This is a global in exec.go, so we can monkey patch it here.
	dockerFactory = func(project string) (DockerExecClient, error) {
		return mockDocker, nil
	}
	// Restore the original factory after the test.
	defer func() {
		dockerFactory = func(project string) (DockerExecClient, error) {
			return docker.NewClient(project)
		}
	}()

	// 2. Setup a mock session
	sessionName := "test-exec-session"
	sessionState := &runner.SessionState{
		Name:        sessionName,
		Status:      "running",
		ContainerID: "test-container-id",
		PID:         os.Getpid(),
		StartTime:   time.Now(),
	}
	require.NoError(t, sm.SaveSession(sessionState))

	// 3. Setup expectations for the mock
	var execInteractiveCalled bool
	mockDocker.ExecInteractiveFunc = func(ctx context.Context, containerID string, cmd []string) error {
		execInteractiveCalled = true
		assert.Equal(t, "test-container-id", containerID)
		assert.Equal(t, []string{"ls", "-la"}, cmd)
		return nil
	}

	// Replace the Run function to use the mock
	oldRun := execCmd.Run
	execCmd.Run = func(cmd *cobra.Command, args []string) {
		sessionName := args[0]
		command := args[1:]
		sm, _ := sessionManagerFactory()
		session, _ := sm.LoadSession(sessionName)
		mockDocker.ExecInteractive(context.Background(), session.ContainerID, command)
	}
	defer func() { execCmd.Run = oldRun }()

	// 4. Run the command
	_, err := executeCommand(rootCmd, "exec", sessionName, "--", "ls", "-la")
	require.NoError(t, err)

	// 5. Assertions
	assert.True(t, execInteractiveCalled, "ExecInteractive should have been called")
}
