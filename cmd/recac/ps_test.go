package main

import (
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
)

func TestPsCmd_NoSessions(t *testing.T) {
	tempDir := t.TempDir()
	sessionDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(sessionDir, 0755)

	oldSessionManager := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionDir)
	}
	defer func() { sessionManagerFactory = oldSessionManager }()

	output, err := executeCommand(rootCmd, "ps")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.Contains(output, "No sessions found.") {
		t.Errorf("expected output to contain 'No sessions found.', got '%s'", output)
	}
}
