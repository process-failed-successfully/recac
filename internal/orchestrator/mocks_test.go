package orchestrator

import (
	"context"
	"log/slog"
	"recac/internal/runner"

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
