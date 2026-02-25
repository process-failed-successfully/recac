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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

func (m *MockDockerClient) RunContainer(ctx context.Context, image, workspace string, binds, env, cmd []string, user string) (string, error) {
	args := m.Called(ctx, image, workspace, binds, env, cmd, user)
	return args.String(0), args.Error(1)
}

func (m *MockDockerClient) RunContainerWithLabels(ctx context.Context, image, workspace string, binds, env, cmd []string, user string, labels map[string]string) (string, error) {
	args := m.Called(ctx, image, workspace, binds, env, cmd, user, labels)
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

func (m *MockDockerClient) ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockDockerClient) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	args := m.Called(ctx, containerID, force)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	args := m.Called(ctx, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDockerClient) WaitContainer(ctx context.Context, containerID string) (int64, error) {
	args := m.Called(ctx, containerID)
	return int64(args.Int(0)), args.Error(1)
}

func (m *MockDockerClient) ImageExists(ctx context.Context, tag string) (bool, error) {
	args := m.Called(ctx, tag)
	return args.Bool(0), args.Error(1)
}

func (m *MockDockerClient) PullImage(ctx context.Context, imageRef string) error {
	args := m.Called(ctx, imageRef)
	return args.Error(0)
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

func (m *MockGitClient) DeleteTag(repoPath, version string) error {
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

func (m *MockGitClient) Run(repoPath string, cmdArgs ...string) (string, error) {
	args := m.Called(repoPath, cmdArgs)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) DeleteLocalBranch(repoPath, branch string) error {
	args := m.Called(repoPath, branch)
	return args.Error(0)
}

func (m *MockGitClient) CreatePR(repoPath, title, body, base string) (string, error) {
	args := m.Called(repoPath, title, body, base)
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

func (m *MockPoller) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestDockerSpawner_Spawn_Success(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// Updated constructor with "Always" to trigger pull
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", mockPoller, "provider", "model", "Always", mockSM, 30, 5, 10)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
		EnvVars: map[string]string{"CUSTOM_VAR": "value"},
	}

	ctx := context.Background()

	// Expect PullImage because Policy is Always
	mockDocker.On("PullImage", ctx, "test-image").Return(nil)

	// Mock expectations
	// Expect RunContainerWithLabels with env containing project ID
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.MatchedBy(func(env []string) bool {
		// Check for env vars in slice
		hasProjectID := false
		for _, e := range env {
			if e == "RECAC_PROJECT_ID=TICKET-1" {
				hasProjectID = true
				break
			}
		}
		return hasProjectID
	}), mock.Anything, "", mock.Anything).Return("container123", nil)

	// Expect WaitContainer
	mockDocker.On("WaitContainer", ctx, "container123").Return(int(0), nil)

	// Verify SaveSession receives session with repo-url
	mockSM.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
		hasRepoURL := false
		for _, arg := range s.Command {
			if arg == "--repo-url" {
				hasRepoURL = true
				break
			}
		}
		return hasRepoURL && s.StartCommitSHA == ""
	})).Return(nil)

	mockSM.On("LoadSession", "TICKET-1").Return(&runner.SessionState{}, nil)

	// This call happens at the END, so it's still there
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("endsha", nil).Once()

	mockSM.On("SaveSession", mock.AnythingOfType("*runner.SessionState")).Return(nil)

	err := spawner.Spawn(ctx, item)

	assert.NoError(t, err)

	mockGit.AssertExpectations(t)
	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}

func TestDockerSpawner_Spawn_RunContainerFails(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", nil, "", "", "Always", mockSM, 30, 5, 10)
	spawner.GitClient = mockGit

	item := WorkItem{ID: "TICKET-1", RepoURL: "https://github.com/test/repo"}
	ctx := context.Background()
	expectedErr := errors.New("run failed")

	mockDocker.On("PullImage", ctx, "test-image").Return(nil)

	// No Clone or StartSHA calls expected
	mockDocker.On("RunContainerWithLabels", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, mock.Anything, "", mock.Anything).Return("", expectedErr)

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
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", "Always", sm, 30, 5, 10)

	injectionItem := WorkItem{
		ID:      "TASK-1\"; echo \"injected",
		RepoURL: "https://github.com/example/repo",
	}

	client.On("PullImage", mock.Anything, "recac-agent:latest").Return(nil)

	// Capture command from RunContainerWithLabels
	var capturedCmd []string
	client.On("RunContainerWithLabels", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd = args.Get(5).([]string) // env=4, cmd=5
	}).Return("container-123", nil)

	// Expect WaitContainer
	client.On("WaitContainer", mock.Anything, "container-123").Return(int(0), nil)

	// Mock SessionManager
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	err := spawner.Spawn(context.Background(), injectionItem)
	assert.NoError(t, err)

	// The command should be stringified and passed to sh -c.
	// We want to ensure the ID is quoted.
	assert.Len(t, capturedCmd, 3)
	assert.Equal(t, "/bin/sh", capturedCmd[0])
	assert.Equal(t, "-c", capturedCmd[1])

	// Check if the ID is quoted in the command string
	// New implementation uses shellquote
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
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", "Always", sm, 30, 5, 10)

	item := WorkItem{
		ID:      "TASK-ENV-TEST",
		RepoURL: "https://github.com/example/repo",
	}

	client.On("PullImage", mock.Anything, "recac-agent:latest").Return(nil)

	// Capture env from RunContainerWithLabels
	var capturedEnv []string
	client.On("RunContainerWithLabels", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedEnv = args.Get(4).([]string) // env=4
	}).Return("container-env", nil)

	client.On("WaitContainer", mock.Anything, "container-env").Return(int(0), nil)

	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	// Check if environment variables are correctly propagated in Env slice
	hasMaxIter := false
	hasFreq := false
	for _, e := range capturedEnv {
		if e == "RECAC_MAX_ITERATIONS=50" {
			hasMaxIter = true
		}
		if e == "RECAC_MANAGER_FREQUENCY=10m" {
			hasFreq = true
		}
	}
	assert.True(t, hasMaxIter, "Should propagate RECAC_MAX_ITERATIONS")
	assert.True(t, hasFreq, "Should propagate RECAC_MANAGER_FREQUENCY")
}

func TestDockerSpawner_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "", "", "Always", nil, 30, 5, 10)

	err := spawner.Cleanup(context.Background(), WorkItem{ID: "test"})
	assert.NoError(t, err)
}

func TestDockerSpawner_GetLogs(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "", "", "Always", nil, 30, 5, 10)

	ctx := context.Background()
	jobID := "TEST-1"

	t.Run("Success", func(t *testing.T) {
		mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
			return true
		})).Return([]types.Container{{ID: "c1"}}, nil).Once()

		mockDocker.On("ContainerLogs", ctx, "c1").Return(io.NopCloser(strings.NewReader("some logs")), nil).Once()

		logs, err := spawner.GetLogs(ctx, jobID)
		assert.NoError(t, err)

		content, _ := io.ReadAll(logs)
		assert.Equal(t, "some logs", string(content))
	})

	t.Run("NoContainer", func(t *testing.T) {
		mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, nil).Once()

		logs, err := spawner.GetLogs(ctx, jobID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active container")
		assert.Nil(t, logs)
	})

	t.Run("ListError", func(t *testing.T) {
		mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, errors.New("list error")).Once()

		logs, err := spawner.GetLogs(ctx, jobID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "list error")
		assert.Nil(t, logs)
	})
}

func TestDockerSpawner_Cancel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", nil, "", "", "Always", nil, 30, 5, 10)

	ctx := context.Background()
	jobID := "TEST-CANCEL"

	t.Run("Success", func(t *testing.T) {
		mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
			return true
		})).Return([]types.Container{{ID: "c1"}}, nil).Once()
		mockDocker.On("StopContainer", ctx, "c1").Return(nil).Once()

		err := spawner.Cancel(ctx, jobID)
		assert.NoError(t, err)
	})

	t.Run("NoContainer", func(t *testing.T) {
		mockDocker.On("ListContainers", ctx, mock.Anything).Return([]types.Container{}, nil).Once()

		err := spawner.Cancel(ctx, jobID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no active container")
	})
}

func TestDockerSpawner_FlagsPropagation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	poller := new(MockPoller)
	sm := new(MockSessionManager)

	maxIter := 55
	freq := 7
	taskMax := 15
	spawner := NewDockerSpawner(logger, client, "recac-agent:latest", "test-project", poller, "gemini", "gemini-pro", "Always", sm, maxIter, freq, taskMax)

	item := WorkItem{
		ID:      "TASK-FLAGS-TEST",
		RepoURL: "https://github.com/example/repo",
	}

	client.On("PullImage", mock.Anything, "recac-agent:latest").Return(nil)

	var capturedCmd []string
	client.On("RunContainerWithLabels", mock.Anything, "recac-agent:latest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		capturedCmd = args.Get(5).([]string) // cmd=5
	}).Return("container-flags", nil)

	client.On("WaitContainer", mock.Anything, "container-flags").Return(int(0), nil)
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	err := spawner.Spawn(context.Background(), item)
	assert.NoError(t, err)

	cmdStr := capturedCmd[2]
	assert.Contains(t, cmdStr, "--max-iterations=55")
	assert.Contains(t, cmdStr, "--manager-frequency=7")
	assert.Contains(t, cmdStr, "--task-max-iterations=15")
}

func TestDockerSpawner_PullPolicy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sm := new(MockSessionManager)
	poller := new(MockPoller)

	// Setup Session Mock
	sm.On("SaveSession", mock.Anything).Return(nil)
	sm.On("LoadSession", mock.Anything).Return(&runner.SessionState{}, nil)

	item := WorkItem{ID: "POLICY-TEST", RepoURL: "http://git"}

	t.Run("Always", func(t *testing.T) {
		client := new(MockDockerClient)
		spawner := NewDockerSpawner(logger, client, "img", "proj", poller, "p", "m", "Always", sm, 30, 5, 10)

		client.On("PullImage", mock.Anything, "img").Return(nil).Once()
		client.On("RunContainerWithLabels", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("c1", nil)
		client.On("WaitContainer", mock.Anything, "c1").Return(int(0), nil)

		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)
		client.AssertExpectations(t)
	})

	t.Run("Never_Exists", func(t *testing.T) {
		client := new(MockDockerClient)
		spawner := NewDockerSpawner(logger, client, "img", "proj", poller, "p", "m", "Never", sm, 30, 5, 10)

		client.On("ImageExists", mock.Anything, "img").Return(true, nil).Once()
		client.On("RunContainerWithLabels", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("c1", nil)
		client.On("WaitContainer", mock.Anything, "c1").Return(int(0), nil)

		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)
		client.AssertExpectations(t)
	})

	t.Run("Never_NotExists", func(t *testing.T) {
		client := new(MockDockerClient)
		spawner := NewDockerSpawner(logger, client, "img", "proj", poller, "p", "m", "Never", sm, 30, 5, 10)

		client.On("ImageExists", mock.Anything, "img").Return(false, nil).Once()
		// Should fail, no PullImage, no RunContainer

		err := spawner.Spawn(context.Background(), item)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found locally and PullPolicy is Never")
		client.AssertExpectations(t)
	})

	t.Run("IfNotPresent_Exists", func(t *testing.T) {
		client := new(MockDockerClient)
		spawner := NewDockerSpawner(logger, client, "img", "proj", poller, "p", "m", "IfNotPresent", sm, 30, 5, 10)

		client.On("ImageExists", mock.Anything, "img").Return(true, nil).Once()
		// No PullImage
		client.On("RunContainerWithLabels", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("c1", nil)
		client.On("WaitContainer", mock.Anything, "c1").Return(int(0), nil)

		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)
		client.AssertExpectations(t)
	})

	t.Run("IfNotPresent_NotExists", func(t *testing.T) {
		client := new(MockDockerClient)
		spawner := NewDockerSpawner(logger, client, "img", "proj", poller, "p", "m", "IfNotPresent", sm, 30, 5, 10)

		client.On("ImageExists", mock.Anything, "img").Return(false, nil).Once()
		client.On("PullImage", mock.Anything, "img").Return(nil).Once()
		client.On("RunContainerWithLabels", mock.Anything, "img", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("c1", nil)
		client.On("WaitContainer", mock.Anything, "c1").Return(int(0), nil)

		err := spawner.Spawn(context.Background(), item)
		assert.NoError(t, err)
		client.AssertExpectations(t)
	})
}
