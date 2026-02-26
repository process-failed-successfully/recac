package main

import (
	"context"
	"recac/internal/runner"
	"recac/internal/tui"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// MockGitHomeClient for Home Test
type MockGitHomeClient struct {
	RepoExistsVal bool
	Branch        string
	Dirty         bool
	CommitMsg     string
	SHA           string
}

func (m *MockGitHomeClient) RepoExists(d string) bool { return m.RepoExistsVal }
func (m *MockGitHomeClient) CurrentBranch(d string) (string, error) { return m.Branch, nil }
func (m *MockGitHomeClient) Run(d string, args ...string) (string, error) {
	if len(args) > 0 {
		if args[0] == "status" {
			if m.Dirty {
				return " M file.go", nil
			}
			return "", nil
		}
		if args[0] == "log" {
			return m.CommitMsg, nil
		}
	}
	return "", nil
}
func (m *MockGitHomeClient) CurrentCommitSHA(d string) (string, error) { return m.SHA, nil }

// Stubs for interface satisfaction
func (m *MockGitHomeClient) DiffStat(w, s, e string) (string, error) { return "", nil }
func (m *MockGitHomeClient) Clone(ctx context.Context, u, d string) error { return nil }
func (m *MockGitHomeClient) Config(d, k, v string) error { return nil }
func (m *MockGitHomeClient) ConfigGlobal(k, v string) error { return nil }
func (m *MockGitHomeClient) ConfigAddGlobal(k, v string) error { return nil }
func (m *MockGitHomeClient) RemoteBranchExists(d, r, b string) (bool, error) { return false, nil }
func (m *MockGitHomeClient) Fetch(d, r, b string) error { return nil }
func (m *MockGitHomeClient) Checkout(d, b string) error { return nil }
func (m *MockGitHomeClient) CheckoutNewBranch(d, b string) error { return nil }
func (m *MockGitHomeClient) Push(d, b string) error { return nil }
func (m *MockGitHomeClient) Pull(d, r, b string) error { return nil }
func (m *MockGitHomeClient) Stash(d string) error { return nil }
func (m *MockGitHomeClient) Merge(d, b string) error { return nil }
func (m *MockGitHomeClient) AbortMerge(d string) error { return nil }
func (m *MockGitHomeClient) Recover(d string) error { return nil }
func (m *MockGitHomeClient) Clean(d string) error { return nil }
func (m *MockGitHomeClient) ResetHard(d, r, b string) error { return nil }
func (m *MockGitHomeClient) StashPop(d string) error { return nil }
func (m *MockGitHomeClient) DeleteRemoteBranch(d, r, b string) error { return nil }
func (m *MockGitHomeClient) Commit(d, msg string) error { return nil }
func (m *MockGitHomeClient) Diff(d, s, e string) (string, error) { return "", nil }
func (m *MockGitHomeClient) DiffStaged(d string) (string, error) { return "", nil }
func (m *MockGitHomeClient) SetRemoteURL(d, n, u string) error { return nil }
func (m *MockGitHomeClient) DeleteLocalBranch(d, b string) error { return nil }
func (m *MockGitHomeClient) LocalBranchExists(d, b string) (bool, error) { return false, nil }
func (m *MockGitHomeClient) Log(d string, args ...string) ([]string, error) { return nil, nil }
func (m *MockGitHomeClient) BisectStart(d, b, g string) error { return nil }
func (m *MockGitHomeClient) BisectGood(d, r string) error { return nil }
func (m *MockGitHomeClient) BisectBad(d, r string) error { return nil }
func (m *MockGitHomeClient) BisectReset(d string) error { return nil }
func (m *MockGitHomeClient) BisectLog(d string) ([]string, error) { return nil, nil }
func (m *MockGitHomeClient) Tag(d, v string) error { return nil }
func (m *MockGitHomeClient) DeleteTag(d, v string) error { return nil }
func (m *MockGitHomeClient) PushTags(d string) error { return nil }
func (m *MockGitHomeClient) LatestTag(d string) (string, error) { return "", nil }
func (m *MockGitHomeClient) CreatePR(d, t, b, base string) (string, error) { return "", nil }

func (m *MockGitHomeClient) StashPush(d, msg string) error { return nil }
func (m *MockGitHomeClient) StashList(d string) ([]string, error) { return nil, nil }
func (m *MockGitHomeClient) StashShow(d, id string) (string, error) { return "", nil }
func (m *MockGitHomeClient) StashApply(d, id string) error { return nil }
func (m *MockGitHomeClient) StashDrop(d, id string) error { return nil }
func (m *MockGitHomeClient) StashClear(d string) error { return nil }

// MockSessionManager
type MockHomeSessionManager struct {
	Sessions []*runner.SessionState
}
func (m *MockHomeSessionManager) ListSessions() ([]*runner.SessionState, error) { return m.Sessions, nil }
func (m *MockHomeSessionManager) SaveSession(s *runner.SessionState) error { return nil }
func (m *MockHomeSessionManager) LoadSession(n string) (*runner.SessionState, error) { return nil, nil }
func (m *MockHomeSessionManager) StopSession(n string) error { return nil }
func (m *MockHomeSessionManager) PauseSession(n string) error { return nil }
func (m *MockHomeSessionManager) ResumeSession(n string) error { return nil }
func (m *MockHomeSessionManager) GetSessionLogs(n string) (string, error) { return "", nil }
func (m *MockHomeSessionManager) GetSessionLogContent(n string, l int) (string, error) { return "", nil }
func (m *MockHomeSessionManager) StartSession(n, g string, c []string, w string) (*runner.SessionState, error) { return nil, nil }
func (m *MockHomeSessionManager) GetSessionPath(n string) string { return "" }
func (m *MockHomeSessionManager) IsProcessRunning(p int) bool { return false }
func (m *MockHomeSessionManager) RemoveSession(n string, f bool) error { return nil }
func (m *MockHomeSessionManager) RenameSession(o, n string) error { return nil }
func (m *MockHomeSessionManager) SessionsDir() string { return "" }
func (m *MockHomeSessionManager) GetSessionGitDiffStat(n string) (string, error) { return "", nil }
func (m *MockHomeSessionManager) ArchiveSession(n string) error { return nil }
func (m *MockHomeSessionManager) UnarchiveSession(n string) error { return nil }
func (m *MockHomeSessionManager) ListArchivedSessions() ([]*runner.SessionState, error) { return nil, nil }

func TestRunHome(t *testing.T) {
	origGit := gitClientFactory
	origSession := sessionManagerFactory
	origTUI := runTUIFunc
	defer func() {
		gitClientFactory = origGit
		sessionManagerFactory = origSession
		runTUIFunc = origTUI
	}()

	gitClientFactory = func() IGitClient {
		return &MockGitHomeClient{
			RepoExistsVal: true,
			Branch:        "feature/dashboard",
			Dirty:         true,
			CommitMsg:     "wip",
			SHA:           "123456789",
		}
	}

	sessionManagerFactory = func() (ISessionManager, error) {
		return &MockHomeSessionManager{
			Sessions: []*runner.SessionState{
				{Name: "session-1", StartTime: time.Now(), Status: "running"},
			},
		}, nil
	}

	var capturedModel tui.HomeModel
	runTUIFunc = func(m tui.HomeModel) error {
		capturedModel = m
		return nil
	}

	cmd := &cobra.Command{}
	if err := runHome(cmd, nil); err != nil {
		t.Fatalf("runHome failed: %v", err)
	}

	if capturedModel.Git.Branch != "feature/dashboard" {
		t.Errorf("Expected branch feature/dashboard, got %s", capturedModel.Git.Branch)
	}
	if !capturedModel.Git.Dirty {
		t.Error("Expected dirty git status")
	}
	if capturedModel.Git.LastCommitMsg != "wip" {
		t.Errorf("Expected commit msg wip, got %s", capturedModel.Git.LastCommitMsg)
	}
	if len(capturedModel.Sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(capturedModel.Sessions))
	} else if capturedModel.Sessions[0].Name != "session-1" {
		t.Errorf("Expected session-1, got %s", capturedModel.Sessions[0].Name)
	}
}
