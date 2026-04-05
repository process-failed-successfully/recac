package utils

import (
	"os/exec"
	"testing"
)

func TestOpenBrowserUnsupported(t *testing.T) {
	// Backup the original execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	// We can't really change runtime.GOOS easily.
	// But we can test if the command fails, it should return an error.
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// return a command that doesn't exist to simulate failure
		return exec.Command("thiscommanddoesnotexistatall")
	}

	err := OpenBrowser("http://localhost:8080")
	if err == nil {
		t.Fatalf("Expected error, got nil")
	}
}
