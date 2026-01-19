package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

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
	mockPoller := newMockPoller(nil)

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
	// Note: Clone and first CurrentCommitSHA are REMOVED as they are now delegated to Agent

	mockDocker.On("RunContainer", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)

	// Verify SaveSession receives session with repo-url
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		hasRepoURL := false
		for _, arg := range s.Command {
			if arg == "--repo-url" {
				hasRepoURL = true
				break
			}
		}
		// Also verify StartCommitSHA is empty
		return hasRepoURL && s.StartCommitSHA == ""
	})).Return(nil)

	// Verify Exec includes git identity and project ID env vars
	mockDocker.On("Exec", mock.Anything, "container123", mock.MatchedBy(func(cmd []string) bool {
		cmdStr := cmd[2] // /bin/sh -c <cmdStr>
		return contains(cmdStr, "export RECAC_PROJECT_ID='TICKET-1'") &&
			contains(cmdStr, "export GIT_AUTHOR_NAME='RECAC Agent'") &&
			contains(cmdStr, "export GIT_AUTHOR_EMAIL='agent@recac.io'")
	})).Return("output", nil)
	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{}, nil)

	// This call happens at the END, so it's still there
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

	// No Clone or StartSHA calls expected
	mockDocker.On("RunContainer", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("", expectedErr)

	err := spawner.Spawn(ctx, item)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run failed")
	mockSM.AssertNotCalled(t, "SaveSession")
}
