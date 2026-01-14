package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCleanCmd(t *testing.T) {
	// 1. Setup Test Environment
	tempDir, err := os.MkdirTemp("", "recac-clean-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create dummy workspaces
	ws1 := filepath.Join(tempDir, "workspace1")
	ws2 := filepath.Join(tempDir, "workspace2")
	ws3 := filepath.Join(tempDir, "workspace3")
	require.NoError(t, os.Mkdir(ws1, 0755))
	require.NoError(t, os.Mkdir(ws2, 0755))
	require.NoError(t, os.Mkdir(ws3, 0755))

	// Create mock sessions
	session1 := &runner.SessionState{Name: "session1", Workspace: ws1, Status: "completed", StartTime: time.Now()}
	session2 := &runner.SessionState{Name: "session2", Workspace: ws2, Status: "running", StartTime: time.Now(), PID: 123}
	session3 := &runner.SessionState{Name: "session3", Workspace: ws3, Status: "stopped", StartTime: time.Now()}

	// 2. Define Test Cases
	testCases := []struct {
		name          string
		args          []string
		sessions      []*runner.SessionState
		mockConfirm   string // What to pipe into stdin for confirmation
		expectErr     string
		expectOutput  string
		checkRemoved  []string // session names that should be removed
		checkKept     []string // session names that should NOT be removed
		checkWsDeleted []string // workspace paths that should be deleted
		checkWsKept    []string // workspace paths that should be kept
	}{
		{
			name:        "Clean single session with confirmation",
			args:        []string{"session1"},
			sessions:    []*runner.SessionState{session1},
			mockConfirm: "y\n",
			expectOutput: "Successfully cleaned session 'session1'",
			checkRemoved:[]string{"session1"},
			checkWsDeleted: []string{ws1},
		},
		{
			name:        "Clean single session decline confirmation",
			args:        []string{"session1"},
			sessions:    []*runner.SessionState{session1},
			mockConfirm: "n\n",
			expectOutput: "Skipping cleanup for session 'session1'",
			checkKept:   []string{"session1"},
			checkWsKept: []string{ws1},
		},
		{
			name:     "Clean single session with force",
			args:     []string{"session1", "--force"},
			sessions: []*runner.SessionState{session1},
			expectOutput: "Successfully cleaned session 'session1'",
			checkRemoved:[]string{"session1"},
			checkWsDeleted: []string{ws1},
		},
		{
			name:      "Clean running session fails without force",
			args:      []string{"session2"},
			sessions:  []*runner.SessionState{session2},
			expectErr: "session 'session2' is running",
			checkKept: []string{"session2"},
			checkWsKept: []string{ws2},
		},
		{
			name:     "Clean running session with force",
			args:     []string{"session2", "--force"},
			sessions: []*runner.SessionState{session2},
			expectOutput: "Successfully cleaned session 'session2'",
			checkRemoved:[]string{"session2"},
			checkWsDeleted: []string{ws2},
		},
		{
			name:     "Clean all completed and stopped sessions",
			args:     []string{"--all", "--force"},
			sessions: []*runner.SessionState{session1, session2, session3},
			expectOutput: "Successfully cleaned session 'session1'",
			checkRemoved:[]string{"session1", "session3"},
			checkKept:   []string{"session2"},
			checkWsDeleted: []string{ws1, ws3},
			checkWsKept:    []string{ws2},
		},
		{
			name:      "Clean non-existent session",
			args:      []string{"non-existent"},
			sessions:  []*runner.SessionState{},
			expectErr: "failed to load session 'non-existent'",
		},
		{
			name:      "No args and no --all flag",
			args:      []string{},
			sessions:  []*runner.SessionState{},
			expectErr: "at least one session name is required, or use the --all flag",
		},
	}

	// 3. Run Test Cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// ARRANGE
			// Recreate workspaces for each test to ensure isolation
			for _, ws := range []string{ws1, ws2, ws3} {
				os.Mkdir(ws, 0755)
			}

			sm := NewMockSessionManager()
			for _, s := range tc.sessions {
				sm.AddSession(s)
			}

			cmd := newCleanCmd(sm)
			b := new(bytes.Buffer)
			cmd.SetOut(b)
			cmd.SetErr(b)
			cmd.SetArgs(tc.args)

			// Mock stdin if needed
			if tc.mockConfirm != "" {
				oldStdin := os.Stdin
				r, w, _ := os.Pipe()
				os.Stdin = r
				w.WriteString(tc.mockConfirm)
				w.Close()
				defer func() { os.Stdin = oldStdin }()
			}

			// ACT
			err := cmd.Execute()

			// ASSERT
			output := b.String()

			if tc.expectErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectErr)
			} else {
				require.NoError(t, err)
			}

			if tc.expectOutput != "" {
				require.Contains(t, output, tc.expectOutput)
			}

			for _, name := range tc.checkRemoved {
				require.True(t, sm.removed[name], "Expected session '%s' to be removed", name)
			}
			for _, name := range tc.checkKept {
				require.False(t, sm.removed[name], "Expected session '%s' to be kept", name)
			}
			for _, ws := range tc.checkWsDeleted {
				_, err := os.Stat(ws)
				require.True(t, os.IsNotExist(err), "Expected workspace '%s' to be deleted", ws)
			}
			for _, ws := range tc.checkWsKept {
				_, err := os.Stat(ws)
				require.NoError(t, err, "Expected workspace '%s' to be kept", ws)
			}
		})
	}
}
