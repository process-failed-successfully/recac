package main

import (
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
)

// setupSearchTest creates a temporary directory with mock session logs.
func setupSearchTest(t *testing.T) (cleanup func(), sessionsDir string) {
	t.Helper()
	sessionsDir = t.TempDir()

	// --- Mock Session 1 ---
	session1Dir := filepath.Join(sessionsDir, "session-1")
	require.NoError(t, os.MkdirAll(session1Dir, 0755))
	log1Path := filepath.Join(sessionsDir, "session-1.log")
	log1Content := `INFO: Starting process
DEBUG: Found value: Apple
ERROR: Process failed with exit code 1
`
	require.NoError(t, os.WriteFile(log1Path, []byte(log1Content), 0644))

	// --- Mock Session 2 ---
	session2Dir := filepath.Join(sessionsDir, "session-2")
	require.NoError(t, os.MkdirAll(session2Dir, 0755))
	log2Path := filepath.Join(sessionsDir, "session-2.log")
	log2Content := `WARN: Deprecated function called.
INFO: All systems nominal. apple.
`
	require.NoError(t, os.WriteFile(log2Path, []byte(log2Content), 0644))

	// --- Mock Session 3 (empty log) ---
	session3Dir := filepath.Join(sessionsDir, "session-3")
	require.NoError(t, os.MkdirAll(session3Dir, 0755))
	log3Path := filepath.Join(sessionsDir, "session-3.log")
	require.NoError(t, os.WriteFile(log3Path, []byte(""), 0644))

	// --- Create corresponding JSON files so ListSessions works ---
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "session-1.json"), []byte(`{"name":"session-1"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "session-2.json"), []byte(`{"name":"session-2"}`), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(sessionsDir, "session-3.json"), []byte(`{"name":"session-3"}`), 0644))

	cleanup = func() {
		os.RemoveAll(sessionsDir)
	}

	return cleanup, sessionsDir
}

func TestSearchLogs(t *testing.T) {
	cleanup, sessionsDir := setupSearchTest(t)
	defer cleanup()

	// Since the mock session manager doesn't have a concept of a directory,
	// we'll test the `doSearchLogs` function directly.
	mockSM := NewMockSessionManager()
	mockSM.SessionsDirFunc = func() string { return sessionsDir }
	mockSM.Sessions = map[string]*runner.SessionState{
		"session-1": {Name: "session-1"},
		"session-2": {Name: "session-2"},
		"session-3": {Name: "session-3"},
	}

	testCases := []struct {
		name          string
		pattern       string
		useRegexp     bool
		caseSensitive bool
		expectedLines []string
		expectedError string
	}{
		{
			name:    "Default Case-Insensitive Search",
			pattern: "apple",
			expectedLines: []string{
				"[session-1] DEBUG: Found value: Apple",
				"[session-2] INFO: All systems nominal. apple.",
			},
		},
		{
			name:          "Case-Sensitive Search - Match",
			pattern:       "Apple",
			caseSensitive: true,
			expectedLines: []string{
				"[session-1] DEBUG: Found value: Apple",
			},
		},
		{
			name:          "Case-Sensitive Search - Partial Match",
			pattern:       "apple",
			caseSensitive: true,
			expectedLines: []string{
				"[session-2] INFO: All systems nominal. apple.",
			},
		},
		{
			name:      "Regex Search",
			pattern:   `^INFO:`,
			useRegexp: true,
			expectedLines: []string{
				"[session-1] INFO: Starting process",
				"[session-2] INFO: All systems nominal. apple.",
			},
		},
		{
			name:      "Regex Search with Word Boundary",
			pattern:   `\bApple\b`,
			useRegexp: true,
			expectedLines: []string{
				"[session-1] DEBUG: Found value: Apple",
			},
		},
		{
			name:          "Invalid Regex",
			pattern:       `[`,
			useRegexp:     true,
			expectedError: "invalid regular expression",
		},
		{
			name:    "No Matches Found",
			pattern: "xyz_no_match",
			expectedLines: []string{
				"No matches found.",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use a dummy cobra command to capture output
			cmd, out, _ := newRootCmd()
			err := doSearchLogs(mockSM, tc.pattern, cmd, tc.useRegexp, tc.caseSensitive)

			if tc.expectedError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
				output := out.String()
				// Normalize line endings and split into lines
				outputLines := strings.Split(strings.TrimSpace(strings.ReplaceAll(output, "\r\n", "\n")), "\n")

				// Handle the case where no lines are expected and output is empty
				if len(tc.expectedLines) == 1 && tc.expectedLines[0] == "" && len(outputLines) == 1 && outputLines[0] == "" {
					// This is a valid empty match
				} else {
					require.ElementsMatch(t, tc.expectedLines, outputLines)
				}
			}
		})
	}
}
