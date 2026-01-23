package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"recac/internal/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"recac/internal/telemetry"
)

// AutoMergeMockJiraClient mocks the JiraClient interface using testify
type AutoMergeMockJiraClient struct {
	mock.Mock
}

func (m *AutoMergeMockJiraClient) AddComment(ctx context.Context, ticketID, comment string) error {
	args := m.Called(ctx, ticketID, comment)
	return args.Error(0)
}

func (m *AutoMergeMockJiraClient) SmartTransition(ctx context.Context, ticketID, targetNameOrID string) error {
	args := m.Called(ctx, ticketID, targetNameOrID)
	return args.Error(0)
}

func TestSession_RunLoop_AutoMerge_Success(t *testing.T) {
	// Setup workspace with git
	tmpDir := t.TempDir()

	// Initialize git repo to satisfy exec.Command calls
	setupGitRepo(t, tmpDir)

	// Setup Mock Git Client
	mockGit := new(MockGitClient)
	// Expect calls
	// 1. Fetch
	mockGit.On("Fetch", tmpDir, "origin", "main").Return(nil).Maybe() // RunLoop calls this in upstream check
	// 1.5 Stash
	mockGit.On("Stash", tmpDir).Return(nil).Maybe()
	// 1.55 Merge origin/main (Upstream sync)
	mockGit.On("Merge", tmpDir, "origin/main").Return(nil).Maybe()
	// 1.6 StashPop
	mockGit.On("StashPop", tmpDir).Return(nil).Maybe()
	// 2. Checkout Base (main)
	mockGit.On("Checkout", tmpDir, "main").Return(nil)
	// 3. Merge Feature (feature-branch)
	// We need to know the feature branch name. setupGitRepo creates 'feature-branch'
	mockGit.On("Merge", tmpDir, "feature-branch").Return(nil)
	// 4. Push Base
	mockGit.On("Push", tmpDir, "main").Return(nil)
	// 5. Delete Remote Branch
	mockGit.On("DeleteRemoteBranch", tmpDir, "origin", "feature-branch").Return(nil)
	// 6. Checkout feature-branch (at end)
	mockGit.On("Checkout", tmpDir, "feature-branch").Return(nil).Maybe()

	// Mock Jira Client
	mockJira := new(AutoMergeMockJiraClient)
	mockJira.On("AddComment", mock.Anything, "JIRA-123", mock.Anything).Return(nil)
	mockJira.On("SmartTransition", mock.Anything, "JIRA-123", "Done").Return(nil)

	// Create Session
	session := &Session{
		Workspace:    tmpDir,
		Project:      "test-project",
		BaseBranch:   "main",
		AutoMerge:    true,
		JiraTicketID: "JIRA-123",
		JiraClient:   mockJira,
		RepoURL:      "http://github.com/test/repo",
		GitClient:    mockGit,
		Logger:       telemetry.NewLogger(true, "", false),
		MaxIterations: 1, // Ensure loop terminates
	}

	// Inject Signals to trigger logic
	signals := map[string]string{
		"PROJECT_SIGNED_OFF": "true",
	}

	dbStore := &TestDBStore{
		Signals: signals,
		MockDBStore: MockDBStore{
			GetFeaturesFunc: func(projectID string) (string, error) {
				return `{"features": [{"id": "1", "status": "done"}]}`, nil
			},
		},
	}
	session.DBStore = dbStore

	// Run Loop
	// RunLoop expects app_spec.txt
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("spec"), 0644)

	err := session.RunLoop(context.Background())

	assert.NoError(t, err)

	mockGit.AssertExpectations(t)
	mockJira.AssertExpectations(t)
}

func TestSession_PushProgress_Mock(t *testing.T) {
	tmpDir := t.TempDir()

	mockGit := new(MockGitClient)
	mockGit.On("RepoExists", tmpDir).Return(true)
	mockGit.On("CurrentBranch", tmpDir).Return("feature-branch", nil)
	mockGit.On("Commit", tmpDir, mock.Anything).Return(nil)
	mockGit.On("Merge", tmpDir, mock.Anything).Return(nil).Maybe() // It tries to merge master/main
	mockGit.On("Push", tmpDir, "feature-branch").Return(nil)

	session := &Session{
		Workspace: tmpDir,
		GitClient: mockGit,
		Logger:    telemetry.NewLogger(true, "", false),
	}

	session.pushProgress(context.Background())

	mockGit.AssertExpectations(t)
}

// Helpers

func setupGitRepo(t *testing.T, dir string) {
	execCmd(t, dir, "git", "init")
	execCmd(t, dir, "git", "config", "user.email", "test@example.com")
	execCmd(t, dir, "git", "config", "user.name", "Test User")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644)
	execCmd(t, dir, "git", "add", ".")
	execCmd(t, dir, "git", "commit", "-m", "Initial commit")
	execCmd(t, dir, "git", "branch", "-m", "main")
	execCmd(t, dir, "git", "checkout", "-b", "feature-branch")
}

func execCmd(t *testing.T, dir string, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to run %s %v: %s", name, args, out)
}

// TestDBStore is a simple map-backed store for testing
type TestDBStore struct {
	Signals map[string]string
	MockDBStore // Embed to inherit no-op methods
}

func (s *TestDBStore) GetSignal(projectID, key string) (string, error) {
	if val, ok := s.Signals[key]; ok {
		return val, nil
	}
	return "", nil
}

func (s *TestDBStore) SetSignal(projectID, key, value string) error {
	if s.Signals == nil {
		s.Signals = make(map[string]string)
	}
	s.Signals[key] = value
	return nil
}

func (s *TestDBStore) DeleteSignal(projectID, key string) error {
	if s.Signals != nil {
		delete(s.Signals, key)
	}
	return nil
}

func (s *TestDBStore) GetFeatures(projectID string) (string, error) {
	// If MockDBStore has function set, use it
	if s.MockDBStore.GetFeaturesFunc != nil {
		return s.MockDBStore.GetFeaturesFunc(projectID)
	}
	return `{"project_name": "test", "features": []}`, nil
}

func (s *TestDBStore) SaveFeatures(projectID, features string) error {
	return nil
}

func (s *TestDBStore) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
    return nil, nil
}
