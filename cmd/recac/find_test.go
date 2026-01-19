package main

import (
	"fmt"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var testSessions = []*runner.SessionState{
	{Name: "session-1", Status: "completed", Command: []string{"run", "feature-a"}, StartTime: time.Now().Add(-2 * time.Hour), Error: ""},
	{Name: "session-2", Status: "running", Command: []string{"run", "feature-b"}, StartTime: time.Now().Add(-1 * time.Hour), Error: ""},
	{Name: "session-3", Status: "error", Command: []string{"run", "feature-c"}, StartTime: time.Now().Add(-30 * time.Minute), Error: "something went wrong"},
	{Name: "session-4", Status: "completed", Command: []string{"fix", "bug-x"}, StartTime: time.Now().Add(-3 * time.Hour), Error: ""},
}

func TestFindCmd(t *testing.T) {
	testCases := []struct {
		name             string
		args             []string
		mockSessions     map[string]*runner.SessionState
		mockDiffStat     func(name string) (string, error)
		expectedOutput   []string
		unexpectedOutput []string
	}{
		{
			name:           "No filters",
			args:           []string{},
			mockSessions:   convertSessionSliceToMap(testSessions),
			expectedOutput: []string{"session-1", "session-2", "session-3", "session-4"},
		},
		{
			name:             "Filter by status 'completed'",
			args:             []string{"--status", "completed"},
			mockSessions:     convertSessionSliceToMap(testSessions),
			expectedOutput:   []string{"session-1", "session-4"},
			unexpectedOutput: []string{"session-2", "session-3"},
		},
		{
			name:             "Filter by goal 'feature'",
			args:             []string{"--goal", "feature"},
			mockSessions:     convertSessionSliceToMap(testSessions),
			expectedOutput:   []string{"session-1", "session-2", "session-3"},
			unexpectedOutput: []string{"session-4"},
		},
		{
			name:             "Filter by error 'wrong'",
			args:             []string{"--error", "wrong"},
			mockSessions:     convertSessionSliceToMap(testSessions),
			expectedOutput:   []string{"session-3"},
			unexpectedOutput: []string{"session-1", "session-2", "session-4"},
		},
		{
			name:         "Filter by file 'main.go'",
			args:         []string{"--file", "main.go"},
			mockSessions: convertSessionSliceToMap(testSessions),
			mockDiffStat: func(name string) (string, error) {
				switch name {
				case "session-1":
					return "1 file changed, 1 insertion(+)\n main.go | 1 +", nil
				case "session-4":
					return "1 file changed, 2 deletions(-)\n other.go | 2 --", nil
				default:
					return "no changes", nil
				}
			},
			expectedOutput:   []string{"session-1"},
			unexpectedOutput: []string{"session-2", "session-3", "session-4"},
		},
		{
			name:             "Filter by since '90m'",
			args:             []string{"--since", "90m"},
			mockSessions:     convertSessionSliceToMap(testSessions),
			expectedOutput:   []string{"session-2", "session-3"},
			unexpectedOutput: []string{"session-1", "session-4"},
		},
		{
			name:           "No results found",
			args:           []string{"--status", "zombie"},
			mockSessions:   convertSessionSliceToMap(testSessions),
			expectedOutput: []string{"No sessions found"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSM := NewMockSessionManager()
			mockSM.Sessions = tc.mockSessions
			mockSM.GetSessionGitDiffStatFunc = tc.mockDiffStat

			// Override the factory
			originalFactory := sessionManagerFactory
			sessionManagerFactory = func() (ISessionManager, error) {
				return mockSM, nil
			}
			defer func() { sessionManagerFactory = originalFactory }()

			output, err := executeCommand(rootCmd, append([]string{"find"}, tc.args...)...)
			assert.NoError(t, err)

			for _, expected := range tc.expectedOutput {
				assert.Contains(t, output, expected, fmt.Sprintf("Output should contain '%s'", expected))
			}
			for _, unexpected := range tc.unexpectedOutput {
				assert.NotContains(t, output, unexpected, fmt.Sprintf("Output should not contain '%s'", unexpected))
			}
		})
	}
}

func convertSessionSliceToMap(sessions []*runner.SessionState) map[string]*runner.SessionState {
	m := make(map[string]*runner.SessionState)
	for _, s := range sessions {
		m[s.Name] = s
	}
	return m
}
