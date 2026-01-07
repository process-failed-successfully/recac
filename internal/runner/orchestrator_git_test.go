package runner

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockDBStore implements db.Store for testing
type MockDBStore struct {
	GetFeaturesFunc func() (string, error)
}

func (m *MockDBStore) Close() error                                     { return nil }
func (m *MockDBStore) SaveObservation(projectID, agentID, content string) error { return nil }
func (m *MockDBStore) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	return nil, nil
}
func (m *MockDBStore) SetSignal(key, value string) error    { return nil }
func (m *MockDBStore) GetSignal(key string) (string, error)             { return "", nil }
func (m *MockDBStore) DeleteSignal(key string) error                    { return nil }
func (m *MockDBStore) SaveFeatures(features string) error               { return nil }
func (m *MockDBStore) GetFeatures() (string, error) {
	if m.GetFeaturesFunc != nil {
		return m.GetFeaturesFunc()
	}
	return "", nil
}
func (m *MockDBStore) UpdateFeatureStatus(id string, status string, passes bool) error { return nil }
func (m *MockDBStore) AcquireLock(path, agentID string, timeout time.Duration) (bool, error) {
	return true, nil
}
func (m *MockDBStore) ReleaseLock(path, agentID string) error { return nil }
func (m *MockDBStore) ReleaseAllLocks(agentID string) error   { return nil }
func (m *MockDBStore) GetActiveLocks() ([]db.Lock, error)     { return nil, nil }
func (m *MockDBStore) Cleanup() error                         { return nil }

func TestOrchestrator_EnsureGitRepo(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "orchestrator_git_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Setenv("GIT_AUTHOR_NAME", "RECAC Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "RECAC Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	// Create a dummy file to commit
	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("hello"), 0644)
	require.NoError(t, err)

	// Mock deps
	mockDocker := &MockDockerClient{}
	mockAgent := &agent.MockAgent{}
	mockDB := &MockDBStore{
		GetFeaturesFunc: func() (string, error) { return "", nil },
	}

	o := NewOrchestrator(mockDB, mockDocker, tmpDir, "img", mockAgent, "proj", "gemini", "gemini-pro", 1, "")

	// Manually call ensureGitRepo (since it's private, we'll verify via Run or reflection,
	// but to test the logic specifically we can temporarily export it or assume Run calls it.
	// Since Run calls it first thing, we can just call Run with a cancelled context or extremely short timeout)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// We expect Run to fail or timeout, but ensureGitRepo should happen before that.
	_ = o.Run(ctx)

	// Check if .git exists
	gitDir := filepath.Join(tmpDir, ".git")
	_, err = os.Stat(gitDir)
	assert.NoError(t, err, ".git directory should exist")

	// Verify commit exists
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	assert.NoError(t, err)
	assert.Contains(t, string(out), "Initial commit (Orchestrator)")
}
