package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"recac/internal/runner"
	"strings"
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

func (m *MockGitClient) Fetch(repoPath, remote, branch string) error {
	args := m.Called(repoPath, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) Tag(repoPath, version string) error {
	args := m.Called(repoPath, version)
	return args.Error(0)
}

func (m *MockGitClient) PushTags(repoPath string) error {
	args := m.Called(repoPath)
	return args.Error(0)
}

func (m *MockGitClient) LatestTag(repoPath string) (string, error) {
	args := m.Called(repoPath)
	return args.String(0), args.Error(1)
}

// Mock Poller (Minimal for this test)
type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	args := m.Called(ctx, logger)
	return args.Get(0).([]WorkItem), args.Error(1)
}

func (m *MockPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	args := m.Called(ctx, item, status, comment)
	return args.Error(0)
}

func TestDockerSpawner_Spawn_Success(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

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
		// Note: RECAC_PROJECT_ID is now quoted with shellquote, so simple strings might not have quotes
		// or use single quotes. 'TICKET-1' (no spaces) -> TICKET-1
		return contains(cmdStr, "export RECAC_PROJECT_ID=TICKET-1") &&
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

func TestDockerSpawner_ShellInjection(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)

	injectionItem := WorkItem{
		ID:      "TASK-1\"; echo \"injected",
		RepoURL: "https://github.com/example/repo",
	}

	client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-123", nil)

	// Mock SessionManager
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	// Capture the command passed to Exec using a channel for synchronization
	capturedCmdChan := make(chan []string, 1)
	client.On("Exec", mock.Anything, "container-123", mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd := args.Get(2).([]string)
		capturedCmdChan <- capturedCmd
	}).Return("Success", nil)

	err := spawner.Spawn(context.Background(), injectionItem)
	assert.NoError(t, err)

	// Wait for the background goroutine to call Exec
	var capturedCmd []string
	select {
	case capturedCmd = <-capturedCmdChan:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	// The command should be stringified and passed to sh -c.
	// We want to ensure the ID is quoted.
	assert.Len(t, capturedCmd, 3)
	assert.Equal(t, "/bin/sh", capturedCmd[0])
	assert.Equal(t, "-c", capturedCmd[1])

	// Check if the ID is quoted in the command string
	// Depending on implementation, checking for quoted ID:
	// New implementation uses shellquote, so it should use single quotes for complex strings
	assert.Contains(t, capturedCmd[2], "--jira 'TASK-1\"; echo \"injected'")
}

func TestDockerSpawner_EnvPropagation(t *testing.T) {
	// Set environment variables for the test process
	os.Setenv("RECAC_MAX_ITERATIONS", "50")
	os.Setenv("RECAC_MANAGER_FREQUENCY", "10m")
	defer os.Unsetenv("RECAC_MAX_ITERATIONS")
	defer os.Unsetenv("RECAC_MANAGER_FREQUENCY")

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", sm)

	item := WorkItem{
		ID:      "TASK-ENV-TEST",
		RepoURL: "https://github.com/example/repo",
	}

	client.On("RunContainer", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("container-env", nil)
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	// Capture the command passed to Exec
	capturedCmdChan := make(chan []string, 1)
	client.On("Exec", mock.Anything, "container-env", mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd := args.Get(2).([]string)
		capturedCmdChan <- capturedCmd
	}).Return("Success", nil)

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	var capturedCmd []string
	select {
	case capturedCmd = <-capturedCmdChan:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for Exec call")
	}

	cmdStr := capturedCmd[2]

	// Check if environment variables are correctly propagated
	assert.Contains(t, cmdStr, "export RECAC_MAX_ITERATIONS=50", "Should propagate RECAC_MAX_ITERATIONS from host")
	assert.Contains(t, cmdStr, "export RECAC_MANAGER_FREQUENCY=10m", "Should propagate RECAC_MANAGER_FREQUENCY from host")
}