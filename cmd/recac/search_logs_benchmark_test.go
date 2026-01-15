package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"recac/internal/runner"
)

func BenchmarkSearchLogsRegex(b *testing.B) {
	// Setup 100 sessions
	sessionsDir := b.TempDir()
	mockSM := NewMockSessionManager()
	mockSM.SessionsDirFunc = func() string { return sessionsDir }

	numSessions := 100
	for i := 0; i < numSessions; i++ {
		name := fmt.Sprintf("session-%d", i)
		// Create log file
		logPath := filepath.Join(sessionsDir, name+".log")
		// Write some content
		content := "INFO: Starting process\nDEBUG: Something happening\nERROR: Oh no\n"
		err := os.WriteFile(logPath, []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}

		mockSM.Sessions[name] = &runner.SessionState{Name: name}
	}

	cmd, _, _ := newRootCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use a simple regex
		_ = doSearchLogs(mockSM, "^ERROR:", cmd, true, false)
	}
}
