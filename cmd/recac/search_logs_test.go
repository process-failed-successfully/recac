package main

import (
	"context"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			matchFunc, err := getMatchFunc(tc.pattern, tc.useRegexp, tc.caseSensitive)
			if tc.expectedError != "" {
				if err != nil {
					require.Contains(t, err.Error(), tc.expectedError)
					return
				}
				// If error was expected but getMatchFunc succeeded, we might fail later or passing invalid regex is caught here.
				// For invalid regex, getMatchFunc returns error.
			}
			require.NoError(t, err)

			// Use a dummy cobra command to capture output
			cmd, out, _ := newRootCmd()
			err = doSearchLogs(mockSM, matchFunc, cmd)

			if tc.expectedError != "" && err != nil {
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

func TestSearchLogs_Remote(t *testing.T) {
	mockK8s := &MockK8sClient{
		ListPodsFunc: func(ctx context.Context, labelSelector string) ([]corev1.Pod, error) {
			return []corev1.Pod{
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "pod-2"}},
			}, nil
		},
		GetPodLogsFunc: func(ctx context.Context, name string, tailLines int64) (string, error) {
			if name == "pod-1" {
				return "INFO: System started\nERROR: Connection failed", nil
			}
			if name == "pod-2" {
				return "DEBUG: Processing request\nINFO: Connection successful", nil
			}
			return "", nil
		},
	}

	testCases := []struct {
		name          string
		pattern       string
		expectedLines []string
	}{
		{
			name:    "Remote Search Match",
			pattern: "Connection",
			expectedLines: []string{
				"[pod-1] ERROR: Connection failed",
				"[pod-2] INFO: Connection successful",
			},
		},
		{
			name:    "Remote Search Specific Pod",
			pattern: "failed",
			expectedLines: []string{
				"[pod-1] ERROR: Connection failed",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matchFunc, err := getMatchFunc(tc.pattern, false, false)
			require.NoError(t, err)

			cmd, out, _ := newRootCmd()
			err = doSearchLogsRemote(mockK8s, matchFunc, cmd)
			require.NoError(t, err)

			output := out.String()
			outputLines := strings.Split(strings.TrimSpace(strings.ReplaceAll(output, "\r\n", "\n")), "\n")
			require.ElementsMatch(t, tc.expectedLines, outputLines)
		})
	}
}
