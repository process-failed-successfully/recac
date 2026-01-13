package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPoller
type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) Poll(ctx context.Context) ([]WorkItem, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]WorkItem), args.Error(1)
}

func (m *MockPoller) Claim(ctx context.Context, item WorkItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockPoller) UpdateStatus(ctx context.Context, item WorkItem, status, message string) error {
	args := m.Called(ctx, item, status, message)
	return args.Error(0)
}

// MockDockerClient
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) RunContainer(ctx context.Context, image, workspace string, extraBinds []string, ports []string, user string) (string, error) {
	args := m.Called(ctx, image, workspace, extraBinds, ports, user)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, cmd)
	return args.String(0), args.Error(1)
}

func TestDockerSpawner_Spawn(t *testing.T) {
	item := WorkItem{
		ID:      "TASK-1",
		RepoURL: "https://github.com/example/repo",
	}

	t.Run("Spawn Success", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		client := new(MockDockerClient)
		poller := new(MockPoller)
		spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro")

		// We pass nil for ports, so the mock should expect that
		client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, []string(nil), mock.AnythingOfType("string")).Return("container-123", nil)

		// Assert that the command passed to Exec contains the correct git config logic
		client.On("Exec", mock.Anything, "container-123", mock.MatchedBy(func(cmd []string) bool {
			// cmd is ["/bin/sh", "-c", "full command string"]
			if len(cmd) != 3 {
				return false
			}
			fullCmd := cmd[2]
			hasGitConfig := assert.Contains(t, fullCmd, `git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"`)
			hasRepoURL := assert.Contains(t, fullCmd, `--repo-url "https://github.com/example/repo"`)
			return hasGitConfig && hasRepoURL
		})).Return("Success", nil)

		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)

		client.AssertExpectations(t)
	})

	t.Run("RunContainer Failure", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		client := new(MockDockerClient)
		poller := new(MockPoller)
		spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro")

		client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("docker error"))

		err := spawner.Spawn(context.Background(), item)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker error")
	})
}
