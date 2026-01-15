package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Docker Client
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) RunContainer(ctx context.Context, image, workspace string, binds, env []string, user string) (string, error) {
	args := m.Called(ctx, image, workspace, binds, env, user)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) StopContainer(ctx context.Context, containerID string) error {
	args := m.Called(ctx, containerID)
	return args.Error(0)
}

func (m *MockDockerClient) Exec(ctx context.Context, containerID string, cmd []string) (string, error) {
	args := m.Called(ctx, containerID, cmd)
	return args.String(0), args.Error(1)
}

// Mock Session Manager
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

// Mock Git Client
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) Clone(ctx context.Context, repoURL, destPath string) error {
	args := m.Called(ctx, repoURL, destPath)
	return args.Error(0)
}

func (m *MockGitClient) CurrentCommitSHA(repoPath string) (string, error) {
	args := m.Called(repoPath)
	return args.String(0), args.Error(1)
}

func TestDockerSpawner_Spawn_Success(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(mockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
		EnvVars: map[string]string{"CUSTOM_VAR": "value"},
	}

	ctx := context.Background()

	// Mock expectations
	mockGit.On("Clone", ctx, item.RepoURL, mock.AnythingOfType("string")).Return(nil)
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("startsha", nil).Once()
	mockDocker.On("RunContainer", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil)
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Return("output", nil)
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{}, nil)
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("endsha", nil).Once()
	mockSM.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil)

	err := spawner.Spawn(ctx, item)

	assert.NoError(t, err)

	// Allow goroutine to run
	time.Sleep(100 * time.Millisecond)

	mockGit.AssertExpectations(t)
	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}

func TestDockerSpawner_Spawn_CloneFails(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", nil, "", "", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{ID: "TICKET-1", RepoURL: "https://github.com/test/repo"}
	ctx := context.Background()
	expectedErr := errors.New("clone failed")

	mockGit.On("Clone", ctx, item.RepoURL, mock.AnythingOfType("string")).Return(expectedErr)

	err := spawner.Spawn(ctx, item)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "clone failed")
	mockDocker.AssertNotCalled(t, "RunContainer")
}

func TestDockerSpawner_Spawn_RunContainerFails(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", nil, "", "", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{ID: "TICKET-1", RepoURL: "https://github.com/test/repo"}
	ctx := context.Background()
	expectedErr := errors.New("run failed")

	mockGit.On("Clone", ctx, item.RepoURL, mock.AnythingOfType("string")).Return(nil)
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("startsha", nil)
	mockDocker.On("RunContainer", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("", expectedErr)

	err := spawner.Spawn(ctx, item)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run failed")
	mockSM.AssertNotCalled(t, "SaveSession")
}
