package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockDockerClient provides a mock implementation of the docker.Client for testing.
type mockDockerClient struct {
	RunContainerFn func(ctx context.Context, image, workspace string, extraBinds []string, env []string, user string) (string, error)
	ExecFn         func(ctx context.Context, containerID string, cmd []string) (string, error)
	CloseFn        func() error

	// Records calls
	execCmd       []string
	runWorkspace  string
	runExtraBinds []string
	runEnv        []string
}

func (m *mockDockerClient) RunContainer(ctx context.Context, image, workspace string, extraBinds []string, env []string, user string) (string, error) {
	m.runWorkspace = workspace
	m.runExtraBinds = extraBinds
	m.runEnv = env
	if m.RunContainerFn != nil {
		return m.RunContainerFn(ctx, image, workspace, extraBinds, env, user)
	}
	return "mock-container-id", nil
}

func (m *mockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	m.execCmd = cmd
	if m.ExecFn != nil {
		return m.ExecFn(ctx, containerID, cmd)
	}
	return "exec success", nil
}

func (m *mockDockerClient) Close() error {
	if m.CloseFn != nil {
		return m.CloseFn()
	}
	return nil
}

// Ensure mockDockerClient implements the required interface subset.
var _ dockerClient = (*mockDockerClient)(nil)

func TestNewDockerSpawner(t *testing.T) {
	client := &mockDockerClient{}
	poller := newMockPoller(nil)
	spawner := NewDockerSpawner(silentLogger, client, "test-image", "test-project", poller, "provider", "model")

	require.NotNil(t, spawner)
	assert.Equal(t, client, spawner.Client)
	assert.Equal(t, "test-image", spawner.Image)
	assert.Equal(t, "test-project", spawner.projectName)
	assert.Equal(t, poller, spawner.Poller)
	assert.Equal(t, "provider", spawner.AgentProvider)
	assert.Equal(t, "model", spawner.AgentModel)
}

func TestDockerSpawner_Spawn_RunContainerError(t *testing.T) {
	mockClient := &mockDockerClient{
		RunContainerFn: func(ctx context.Context, image, workspace string, extraBinds []string, env []string, user string) (string, error) {
			// Check that the temp dir was created before this point
			assert.DirExists(t, workspace)
			return "", errors.New("container failed")
		},
	}
	spawner := NewDockerSpawner(silentLogger, mockClient, "img", "proj", nil, "", "")

	item := WorkItem{ID: "TEST-1"}
	err := spawner.Spawn(context.Background(), item)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start container")
	// Verify the workspace was cleaned up
	assert.NoDirExists(t, mockClient.runWorkspace)
}

func TestDockerSpawner_Spawn_ExecLogic(t *testing.T) {
	// Set secrets in the test environment
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("JIRA_API_TOKEN", "jira-token")

	testCases := []struct {
		name               string
		execErr            error
		expectedStatus     string
		expectStatusUpdate bool
	}{
		{
			name:               "Exec Fails",
			execErr:            errors.New("exec failed"),
			expectedStatus:     "Failed",
			expectStatusUpdate: true,
		},
		{
			name:               "Exec Succeeds",
			execErr:            nil,
			expectedStatus:     "", // No update
			expectStatusUpdate: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var wg sync.WaitGroup
			wg.Add(1)

			mockClient := &mockDockerClient{
				ExecFn: func(ctx context.Context, containerID string, cmd []string) (string, error) {
					defer wg.Done()
					return "output", tc.execErr
				},
			}
			poller := newMockPoller(nil)
			spawner := NewDockerSpawner(silentLogger, mockClient, "test-image", "test-proj", poller, "test-provider", "test-model")

			item := WorkItem{
				ID:      "TEST-EXEC-1",
				RepoURL: "https://github.com/user/repo.git",
				EnvVars: map[string]string{"CUSTOM_VAR": "custom_value"},
			}

			err := spawner.Spawn(context.Background(), item)
			require.NoError(t, err)

			// Wait for the background goroutine to call Exec
			wg.Wait()

			// Check status update
			poller.updateStatusMu.Lock()
			if tc.expectStatusUpdate {
				assert.Equal(t, tc.expectedStatus, poller.updateStatus[item.ID])
			} else {
				assert.Empty(t, poller.updateStatus)
			}
			poller.updateStatusMu.Unlock()

			// Check that the workspace was created and not deleted
			assert.DirExists(t, mockClient.runWorkspace)
			// Manually clean up the temp dir for the test
			defer t.Cleanup(func() {
				_ = os.RemoveAll(mockClient.runWorkspace)
			})

			// Check that the shell command is correct
			require.Len(t, mockClient.execCmd, 3)
			assert.Equal(t, "/bin/sh", mockClient.execCmd[0])
			assert.Equal(t, "-c", mockClient.execCmd[1])

			cmdStr := mockClient.execCmd[2]
			assert.Contains(t, cmdStr, "cd /workspace")
			assert.NotContains(t, cmdStr, "export ")
			assert.Contains(t, cmdStr, "git clone --depth 1 https://test-token@github.com/user/repo.git .")
			assert.Contains(t, cmdStr, "/usr/local/bin/recac start --jira TEST-EXEC-1")

			// Check env vars passed to container
			assert.Contains(t, mockClient.runEnv, "RECAC_PROVIDER=test-provider")
			assert.Contains(t, mockClient.runEnv, "RECAC_MODEL=test-model")
			assert.Contains(t, mockClient.runEnv, "CUSTOM_VAR=custom_value")
			assert.Contains(t, mockClient.runEnv, "JIRA_API_TOKEN=jira-token")
			assert.Contains(t, mockClient.runEnv, fmt.Sprintf("RECAC_HOST_WORKSPACE_PATH=%s", mockClient.runWorkspace))
		})
	}
}

func TestDockerSpawner_Cleanup(t *testing.T) {
	spawner := NewDockerSpawner(silentLogger, nil, "", "", nil, "", "")
	err := spawner.Cleanup(context.Background(), WorkItem{})
	assert.NoError(t, err, "Cleanup should be a no-op and not return an error")
}
