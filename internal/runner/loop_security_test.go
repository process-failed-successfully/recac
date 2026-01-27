package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/git"
	"recac/internal/telemetry"
	"testing"
	"time"
)

// FullMockGitClient implements git.IClient with all methods
type FullMockGitClient struct {
	CommitFunc func(directory, message string) error
	// Add other fields as needed for the test
}

func (m *FullMockGitClient) DiffStat(workspace, startCommit, endCommit string) (string, error) { return "", nil }
func (m *FullMockGitClient) CurrentCommitSHA(workspace string) (string, error) { return "sha", nil }
func (m *FullMockGitClient) Clone(ctx context.Context, repoURL, directory string) error { return nil }
func (m *FullMockGitClient) RepoExists(directory string) bool { return true }
func (m *FullMockGitClient) Config(directory, key, value string) error { return nil }
func (m *FullMockGitClient) ConfigGlobal(key, value string) error { return nil }
func (m *FullMockGitClient) ConfigAddGlobal(key, value string) error { return nil }
func (m *FullMockGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) { return true, nil }
func (m *FullMockGitClient) Fetch(directory, remote, branch string) error { return nil }
func (m *FullMockGitClient) Checkout(directory, branch string) error { return nil }
func (m *FullMockGitClient) CheckoutNewBranch(directory, branch string) error { return nil }
func (m *FullMockGitClient) Push(directory, branch string) error { return nil }
func (m *FullMockGitClient) Pull(directory, remote, branch string) error { return nil }
func (m *FullMockGitClient) Stash(directory string) error { return nil }
func (m *FullMockGitClient) Merge(directory, branchName string) error { return nil }
func (m *FullMockGitClient) AbortMerge(directory string) error { return nil }
func (m *FullMockGitClient) Recover(directory string) error { return nil }
func (m *FullMockGitClient) Clean(directory string) error { return nil }
func (m *FullMockGitClient) ResetHard(directory, remote, branch string) error { return nil }
func (m *FullMockGitClient) StashPop(directory string) error { return nil }
func (m *FullMockGitClient) DeleteRemoteBranch(directory, remote, branch string) error { return nil }
func (m *FullMockGitClient) CurrentBranch(directory string) (string, error) { return "feature-branch", nil }
func (m *FullMockGitClient) Commit(directory, message string) error {
	if m.CommitFunc != nil {
		return m.CommitFunc(directory, message)
	}
	return nil
}
func (m *FullMockGitClient) Diff(directory, startCommit, endCommit string) (string, error) { return "", nil }
func (m *FullMockGitClient) DiffStaged(directory string) (string, error) { return "", nil }
func (m *FullMockGitClient) SetRemoteURL(directory, name, url string) error { return nil }
func (m *FullMockGitClient) DeleteLocalBranch(directory, branch string) error { return nil }
func (m *FullMockGitClient) LocalBranchExists(directory, branch string) (bool, error) { return false, nil }
func (m *FullMockGitClient) Log(directory string, args ...string) ([]string, error) { return []string{}, nil }
func (m *FullMockGitClient) BisectStart(directory, bad, good string) error { return nil }
func (m *FullMockGitClient) BisectGood(directory, rev string) error { return nil }
func (m *FullMockGitClient) BisectBad(directory, rev string) error { return nil }
func (m *FullMockGitClient) BisectReset(directory string) error { return nil }
func (m *FullMockGitClient) BisectLog(directory string) ([]string, error) { return []string{}, nil }
func (m *FullMockGitClient) Tag(directory, version string) error { return nil }
func (m *FullMockGitClient) DeleteTag(directory, version string) error { return nil }
func (m *FullMockGitClient) PushTags(directory string) error { return nil }
func (m *FullMockGitClient) LatestTag(directory string) (string, error) { return "", nil }
func (m *FullMockGitClient) Run(directory string, args ...string) (string, error) { return "", nil }

func TestVulnerability_ExecCommandInjection_Fixed(t *testing.T) {
	// Setup temporary workspace
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "app_spec.txt"), []byte("spec"), 0644)

	// Create a malicious project name
	maliciousProject := "foo'; rm -rf /tmp/test; echo '"

	// Setup session
	session := &Session{
		Workspace:    tmpDir,
		Project:      maliciousProject,
		AutoMerge:    true,
		BaseBranch:   "main",
		Logger:       telemetry.NewLogger(true, "", false),
		SleepFunc:    func(d time.Duration) {}, // Fast sleep
		MaxIterations: 1,
	}

	// Pre-condition: PROJECT_SIGNED_OFF to trigger auto-merge logic
	session.DBStore = nil // No DB, using memory signals if possible, or we need DB to set signals?
	// Session has internal method createSignal but it might depend on DB.
	// Let's check hasSignal implementation.
	// It uses DBStore.GetSignal if DBStore is present.
	// If DBStore is nil, it seems RunLoop will just fail if it relies on persistent signals?
	// Wait, session doesn't store signals in memory struct?

	// Let's look at Session struct in internal/runner/session.go
	// Signals seem to be DB only concept?
	// func (s *Session) hasSignal(name string) bool { ... }

	// If I mock DBStore, I can control signals.
	mockDB := &FaultToleranceMockDB{
		Signals: map[string]string{
			"PROJECT_SIGNED_OFF": "true",
		},
	}
	session.DBStore = mockDB

	// Override Git Client
	commitCalled := false
	mockGit := &FullMockGitClient{
		CommitFunc: func(directory, message string) error {
			commitCalled = true
			expectedMsg := "feat: implemented features for " + maliciousProject
			if message != expectedMsg {
				t.Errorf("Commit message mismatch. Expected: %s, Got: %s", expectedMsg, message)
			}
			return nil
		},
	}

	originalNewClient := git.NewClient
	git.NewClient = func() git.IClient {
		return mockGit
	}
	defer func() { git.NewClient = originalNewClient }()

	// We need to ensure RunLoop doesn't crash on other things.
	// RunLoop calls s.Notifier.Notify.
	// session.Notifier is initialized in RunLoop if nil.

	// RunLoop calls loadFeatures.
	// We need features to be loaded and passing?
	// The auto-merge block is executed IF PROJECT_SIGNED_OFF is present.
	// It happens inside the loop.

	// We also need to avoid "premature project sign-off detected".
	// This check happens AFTER the git merge logic.
	// Wait:
	// if s.hasSignal("PROJECT_SIGNED_OFF") {
	//    if s.BaseBranch != "" { ... git logic ... }
	//    features := s.loadFeatures()
	//    ... check incomplete features ...

	// So Git Logic happens BEFORE the feature check. Good.

	// Run
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := session.RunLoop(ctx)

	// We expect the loop to exit or return error (e.g. context deadline or cleaner agent error)
	// We just care if commitCalled is true.

	if !commitCalled {
		t.Error("git.Client.Commit was NOT called. The code is likely still using exec.Command directly.")
	} else {
		t.Log("Success: git.Client.Commit was called with safe arguments.")
	}

	if err != nil && err != context.DeadlineExceeded {
		t.Logf("RunLoop finished with error: %v (expected)", err)
	}
}
