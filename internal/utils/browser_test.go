package utils

import (
	"os/exec"
	"runtime"
	"testing"
)

func TestOpenBrowser(t *testing.T) {
	// Backup the original execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var executedCommand string

	// Mock execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		executedCommand = name
		// Create a dummy command that won't actually do anything
		return exec.Command("echo") // using a harmless command
	}

	err := OpenBrowser("http://localhost:8080")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		if executedCommand != "xdg-open" {
			t.Errorf("Expected xdg-open, got %s", executedCommand)
		}
	case "windows":
		if executedCommand != "rundll32" {
			t.Errorf("Expected rundll32, got %s", executedCommand)
		}
	case "darwin":
		if executedCommand != "open" {
			t.Errorf("Expected open, got %s", executedCommand)
		}
	}
}
