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
	corev1 "k8s.io/api/core/v1"
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

func (m *MockDockerClient) ImageExistsLocally(ctx context.Context, imageName string) (bool, error) {
	args := m.Called(ctx, imageName)
	return args.Bool(0), args.Error(1)
}

func (m *MockDockerClient) PullImage(ctx context.Context, imageName string) error {
	args := m.Called(ctx, imageName)
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

func TestDockerSpawner_Spawn_Success(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(mockPoller)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", mockPoller, "provider", "model", mockSM, corev1.PullAlways)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
		EnvVars: map[string]string{"CUSTOM_VAR": "value"},
	}

	ctx := context.Background()

	// Mock expectations
	mockDocker.On("ImageExistsLocally", ctx, "test-image").Return(true, nil)
	mockDocker.On("PullImage", ctx, "test-image").Return(nil)
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
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", nil, "", "", mockSM, corev1.PullAlways)
	spawner.GitClient = mockGit

	item := WorkItem{ID: "TICKET-1", RepoURL: "https://github.com/test/repo"}
	ctx := context.Background()
	expectedErr := errors.New("clone failed")

	mockDocker.On("ImageExistsLocally", ctx, "test-image").Return(true, nil)
	mockDocker.On("PullImage", ctx, "test-image").Return(nil)
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
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", nil, "", "", mockSM, corev1.PullAlways)
	spawner.GitClient = mockGit

	item := WorkItem{ID: "TICKET-1", RepoURL: "https://github.com/test/repo"}
	ctx := context.Background()
	expectedErr := errors.New("run failed")

	mockDocker.On("ImageExistsLocally", ctx, "test-image").Return(true, nil)
	mockDocker.On("PullImage", ctx, "test-image").Return(nil)
	mockGit.On("Clone", ctx, item.RepoURL, mock.AnythingOfType("string")).Return(nil)
	mockGit.On("CurrentCommitSHA", mock.AnythingOfType("string")).Return("startsha", nil)
	mockDocker.On("RunContainer", ctx, "test-image", mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("", expectedErr)

	err := spawner.Spawn(ctx, item)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "run failed")
	mockSM.AssertNotCalled(t, "SaveSession")
}

func TestDockerSpawner_ensureImage(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	imageName := "test-image:latest"

	testCases := []struct {
		name          string
		pullPolicy    corev1.PullPolicy
		imageExists   bool
		setupMocks    func(*MockDockerClient)
		expectPull    bool
		expectError   bool
		errorContains string
	}{
		{
			name:        "PullAlways_ImageExists",
			pullPolicy:  corev1.PullAlways,
			imageExists: true,
			setupMocks: func(m *MockDockerClient) {
				m.On("ImageExistsLocally", ctx, imageName).Return(true, nil)
				m.On("PullImage", ctx, imageName).Return(nil)
			},
			expectPull:  true,
			expectError: false,
		},
		{
			name:        "PullAlways_ImageMissing",
			pullPolicy:  corev1.PullAlways,
			imageExists: false,
			setupMocks: func(m *MockDockerClient) {
				m.On("ImageExistsLocally", ctx, imageName).Return(false, nil)
				m.On("PullImage", ctx, imageName).Return(nil)
			},
			expectPull:  true,
			expectError: false,
		},
		{
			name:        "PullNever_ImageExists",
			pullPolicy:  corev1.PullNever,
			imageExists: true,
			setupMocks: func(m *MockDockerClient) {
				m.On("ImageExistsLocally", ctx, imageName).Return(true, nil)
			},
			expectPull:  false,
			expectError: false,
		},
		{
			name:        "PullNever_ImageMissing",
			pullPolicy:  corev1.PullNever,
			imageExists: false,
			setupMocks: func(m *MockDockerClient) {
				m.On("ImageExistsLocally", ctx, imageName).Return(false, nil)
			},
			expectPull:    false,
			expectError:   true,
			errorContains: "not found locally and pull policy is 'Never'",
		},
		{
			name:        "PullIfNotPresent_ImageExists",
			pullPolicy:  corev1.PullIfNotPresent,
			imageExists: true,
			setupMocks: func(m *MockDockerClient) {
				m.On("ImageExistsLocally", ctx, imageName).Return(true, nil)
			},
			expectPull:  false,
			expectError: false,
		},
		{
			name:        "PullIfNotPresent_ImageMissing",
			pullPolicy:  corev1.PullIfNotPresent,
			imageExists: false,
			setupMocks: func(m *MockDockerClient) {
				m.On("ImageExistsLocally", ctx, imageName).Return(false, nil)
				m.On("PullImage", ctx, imageName).Return(nil)
			},
			expectPull:  true,
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockDocker := new(MockDockerClient)
			tc.setupMocks(mockDocker)

			spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", nil, "", "", nil, tc.pullPolicy)

			err := spawner.ensureImage(ctx)

			if tc.expectError {
				assert.Error(t, err)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			if tc.expectPull {
				mockDocker.AssertCalled(t, "PullImage", ctx, imageName)
			} else {
				mockDocker.AssertNotCalled(t, "PullImage")
			}
		})
	}
}
