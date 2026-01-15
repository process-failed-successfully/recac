package main

import (
	"fmt"
	"recac/internal/runner"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockGitClient allows us to control git status for tests.
type mockGitClient struct {
	isDirty bool
	err     error
}

func (m *mockGitClient) IsDirty(path string) (bool, error) {
	return m.isDirty, m.err
}
func (m *mockGitClient) CurrentCommitSHA(path string) (string, error)            { return "", nil }
func (m_ *mockGitClient) Diff(workspace, from, to string) (string, error)       { return "", nil }
func (m *mockGitClient) DiffStat(workspace, from, to string) (string, error)     { return "", nil }
func (m *mockGitClient) Checkout(repoPath, commitOrBranch string) error          { return nil }

// mockSessionManager allows us to control session states for tests.
type mockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}
func (m *mockSessionManager) LoadSession(name string) (*runner.SessionState, error)   { return nil, nil }
func (m *mockSessionManager) GetSessionLogs(name string) (string, error)               { return "", nil }
func (m *mockSessionManager) GetSessionGitDiffStat(name string) (string, error)         { return "", nil }
func (m *mockSessionManager) RemoveSession(name string, force bool) error              { return nil }
func (m *mockSessionManager) StopSession(name string) error                            { return nil }
func (m *mockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *mockSessionManager) SaveSession(session *runner.SessionState) error                 { return nil }
func (m *mockSessionManager) GetSessionPath(name string) string                              { return "" }
func (m *mockSessionManager) IsProcessRunning(pid int) bool                                  { return false }
func (m *mockSessionManager) RenameSession(oldName, newName string) error                    { return nil }
func (m *mockSessionManager) SessionsDir() string                                            { return "" }
func (m *mockSessionManager) ArchiveSession(name string) error                               { return nil }
func (m *mockSessionManager) UnarchiveSession(name string) error                             { return nil }
func (m *mockSessionManager) ListArchivedSessions() ([]*runner.SessionState, error)          { return nil, nil }

func TestSuggestCmd(t *testing.T) {
	// --- Test Cases ---
	testCases := []struct {
		name                 string
		isDirty              bool
		gitErr               error
		sessions             []*runner.SessionState
		sessionErr           error
		expectedSubstrings   []string
		unexpectedSubstrings []string
		expectErr            bool
	}{
		{
			name:    "Clean State, No Sessions",
			isDirty: false,
			sessions: []*runner.SessionState{},
			expectedSubstrings: []string{
				"recac start --goal",
				"No running sessions and a clean git status",
			},
			unexpectedSubstrings: []string{"recac ps", "recac show"},
		},
		{
			name:    "Dirty State, No Sessions",
			isDirty: true,
			sessions: []*runner.SessionState{},
			expectedSubstrings: []string{
				"recac start --goal",
				"You have uncommitted changes",
			},
		},
		{
			name:    "Clean State, One Running Session",
			isDirty: false,
			sessions: []*runner.SessionState{
				{Name: "test-running", Status: "running"},
			},
			expectedSubstrings: []string{
				"recac ps",
				"recac attach test-running",
				"You have 1 running session(s)",
			},
		},
		{
			name:    "Clean State, One Completed Session",
			isDirty: false,
			sessions: []*runner.SessionState{
				{Name: "test-done", Status: "completed"},
			},
			expectedSubstrings: []string{
				"recac show test-done",
				"recac prune",
				"Review the work",
			},
		},
		{
			name: "Dirty State, Running and Completed Sessions",
			isDirty: true,
			sessions: []*runner.SessionState{
				{Name: "test-running", Status: "running"},
				{Name: "test-done", Status: "completed"},
			},
			expectedSubstrings: []string{
				"recac start --goal", // From dirty git
				"recac ps",           // From running session
				"recac show test-done", // From completed session
				"recac prune",        // From completed session
			},
		},
		{
			name:       "Session Manager Error",
			isDirty:    false,
			sessionErr: fmt.Errorf("disk is full"),
			expectedSubstrings: []string{
				"could not list sessions: disk is full",
			},
			expectErr: true,
		},
		{
			name:    "More than 5 sessions",
			isDirty: false,
			sessions: []*runner.SessionState{
				{Name: "test-1", Status: "completed"},
				{Name: "test-2", Status: "completed"},
				{Name: "test-3", Status: "completed"},
				{Name: "test-4", Status: "completed"},
				{Name: "test-5", Status: "completed"},
				{Name: "test-6", Status: "completed"},
			},
			expectedSubstrings: []string{"recac ls", "You have many sessions"},
		},
	}

	// --- Run Tests ---
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// --- Mock Dependencies ---
			originalGitClientFactory := gitClientFactory
			originalSessionManagerFactory := sessionManagerFactory
			defer func() {
				gitClientFactory = originalGitClientFactory
				sessionManagerFactory = originalSessionManagerFactory
			}()

			gitClientFactory = func() IGitClient {
				return &mockGitClient{isDirty: tc.isDirty, err: tc.gitErr}
			}
			sessionManagerFactory = func() (ISessionManager, error) {
				// The factory itself should not fail.
				// It should return a mock that is configured to fail on ListSessions if tc.sessionErr is set.
				return &mockSessionManager{sessions: tc.sessions, err: tc.sessionErr}, nil
			}

			// --- Execute Command ---
			root, _, _ := newRootCmd()
			output, err := executeCommand(root, "suggest")

			// --- Assertions ---
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedSubstrings[0])
				return
			}

			require.NoError(t, err)
			for _, sub := range tc.expectedSubstrings {
				assert.Contains(t, output, sub)
			}
			for _, sub := range tc.unexpectedSubstrings {
				assert.NotContains(t, output, sub)
			}
		})
	}
}
