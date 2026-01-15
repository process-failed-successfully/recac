package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockPoller
type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	args := m.Called(ctx, logger)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]WorkItem), args.Error(1)
}

func (m *MockPoller) UpdateStatus(ctx context.Context, item WorkItem, status, message string) error {
	args := m.Called(ctx, item, status, message)
	return args.Error(0)
}

// MockDockerClient
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) RunContainer(ctx context.Context, imageRef string, workspace string, extraBinds []string, ports []string, user string) (string, error) {
	args := m.Called(ctx, imageRef, workspace, extraBinds, ports, user)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, cmd)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

// MockSessionManager
type MockSessionManager struct {
	mock.Mock
}

func (m *MockSessionManager) SaveSession(session *runner.SessionState) error {
	args := m.Called(session)
	return args.Error(0)
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	args := m.Called(name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

func (m *MockSessionManager) GetSessionLogContent(name string, lines int) (string, error) {
	args := m.Called(name, lines)
	return args.String(0), args.Error(1)
}

func (m *MockSessionManager) GetSessionGitDiffStat(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	args := m.Called(name, command, workspace)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*runner.SessionState), args.Error(1)
}

// MockGitClient
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) Clone(ctx context.Context, repoURL, path string) error {
	args := m.Called(ctx, repoURL, path)
	return args.Error(0)
}

func (m *MockGitClient) CurrentCommitSHA(path string) (string, error) {
	args := m.Called(path)
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
		sm := new(MockSessionManager)
		gitClient := new(MockGitClient)
		spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)
		spawner.GitClient = gitClient

		// Mock expectations
		gitClient.On("Clone", mock.Anything, item.RepoURL, mock.AnythingOfType("string")).Return(nil)
		gitClient.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("start-sha", nil)
		client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything).Return("container-123", nil)
		sm.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil)
		client.On("Exec", mock.Anything, "container-123", mock.Anything).Return("Success", nil)
		sm.On("LoadSession", "TASK-1").Return(&runner.SessionState{Name: "TASK-1"}, nil)

		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)

		// Assert that the mocks were called
		gitClient.AssertCalled(t, "Clone", mock.Anything, item.RepoURL, mock.AnythingOfType("string"))
		client.AssertCalled(t, "RunContainer", mock.Anything, "recac-agent:latest", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything)
		sm.AssertCalled(t, "SaveSession", mock.AnythingOfType("*runner.SessionState"))
	})

	t.Run("Git Clone Failure", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		client := new(MockDockerClient)
		poller := new(MockPoller)
		sm := new(MockSessionManager)
		gitClient := new(MockGitClient)
		spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)
		spawner.GitClient = gitClient

		gitClient.On("Clone", mock.Anything, item.RepoURL, mock.AnythingOfType("string")).Return(errors.New("git clone failed"))

		err := spawner.Spawn(context.Background(), item)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "git clone failed")

		client.AssertNotCalled(t, "RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		sm.AssertNotCalled(t, "SaveSession", mock.Anything)
	})

	t.Run("RunContainer Failure", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		client := new(MockDockerClient)
		poller := new(MockPoller)
		sm := new(MockSessionManager)
		gitClient := new(MockGitClient)
		spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)
		spawner.GitClient = gitClient

		gitClient.On("Clone", mock.Anything, item.RepoURL, mock.AnythingOfType("string")).Return(nil)
		gitClient.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("start-sha", nil)
		client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("docker error"))

		err := spawner.Spawn(context.Background(), item)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "docker error")

		sm.AssertNotCalled(t, "SaveSession", mock.Anything)
	})

	t.Run("SaveSession Failure", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
		client := new(MockDockerClient)
		poller := new(MockPoller)
		sm := new(MockSessionManager)
		gitClient := new(MockGitClient)
		spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)
		spawner.GitClient = gitClient

		gitClient.On("Clone", mock.Anything, item.RepoURL, mock.AnythingOfType("string")).Return(nil)
		gitClient.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("start-sha", nil)
		client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything).Return("container-123", nil)
		sm.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(errors.New("failed to save session"))
		client.On("StopContainer", mock.Anything, "container-123").Return(nil)

		err := spawner.Spawn(context.Background(), item)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to save session")

		client.AssertCalled(t, "RunContainer", mock.Anything, "recac-agent:latest", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything)
		sm.AssertCalled(t, "SaveSession", mock.AnythingOfType("*runner.SessionState"))
		client.AssertCalled(t, "StopContainer", mock.Anything, "container-123")
	})
}
