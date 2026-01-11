
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLogsCmd(t *testing.T) {
	// 1. Setup
	tempDir := t.TempDir()
	sessionName := "test-logs-session"
	logFile := filepath.Join(tempDir, sessionName+".log")
	logContent := "line 1\nline 2\nline 3"

	// Create a dummy log file
	err := os.WriteFile(logFile, []byte(logContent), 0600)
	require.NoError(t, err)

	// Create a mock session manager and session
	mockSM, err := runner.NewSessionManagerWithDir(tempDir)
	require.NoError(t, err)

	session := &runner.SessionState{
		Name:    sessionName,
		LogFile: logFile,
		Status:  "completed",
	}
	err = mockSM.SaveSession(session)
	require.NoError(t, err)

	// Override the factory to return our mock
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("logs without follow", func(t *testing.T) {
		// 2. Execute
		rootCmd, _, _ := newRootCmd()
		output, err := executeCommand(rootCmd, "logs", sessionName)

		// 3. Assert
		require.NoError(t, err)
		require.Equal(t, logContent+"\n", output)
	})

	t.Run("logs with follow", func(t *testing.T) {
		// 2. Execute
		rootCmd, _, _ := newRootCmd()
		cmd, outBuff, errBuff := newCommand("logs", sessionName, "-f")
		rootCmd.AddCommand(cmd)

		go func() {
			err := cmd.Execute()
			require.NoError(t, err)
		}()

		// Give the command a moment to start tailing
		time.Sleep(500 * time.Millisecond)

		// Append to the log file
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0600)
		require.NoError(t, err)
		defer f.Close()

		appendedContent := "\nline 4"
		_, err = f.WriteString(appendedContent)
		require.NoError(t, err)

		// Give tail a moment to pick up the change
		time.Sleep(500 * time.Millisecond)

		// 3. Assert
		require.Empty(t, errBuff.String())
		expectedOutput := fmt.Sprintf("%s\n%s\n", logContent, "line 4")
		require.Equal(t, expectedOutput, outBuff.String())
	})

	t.Run("logs for non-existent session", func(t *testing.T) {
		// 2. Execute
		rootCmd, _, _ := newRootCmd()
		_, err := executeCommand(rootCmd, "logs", "non-existent-session")

		// 3. Assert
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to read session file")
	})
}
