package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFeatureStartCmd(t *testing.T) {
	// Setup git repo
	tmpDir := t.TempDir()

	// Mock CWD
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "you@example.com").Run()
	exec.Command("git", "config", "user.name", "Your Name").Run()

	// commit initial
	os.WriteFile("init", []byte("init"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "init").Run()

	// Mock Exit
	oldExit := exit
	defer func() { exit = oldExit }()
	exit = func(code int) {
		t.Fatalf("Unexpected exit with code %d", code)
	}

	// Execute command
	// We need to use RunE if it returns error, but it is Run which doesn't.
	// But featureStartCmd variable is accessible.

	featureStartCmd.Run(featureStartCmd, []string{"my-feature"})

	// Verify branch exists and is checked out
	out, _ := exec.Command("git", "branch", "--show-current").Output()
	branch := strings.TrimSpace(string(out))
	if branch != "feature/my-feature" {
		t.Errorf("Expected branch feature/my-feature, got %s", branch)
	}
}

func TestFeatureStartCmd_Error(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// No git init, so git command fails

	exited := false
	oldExit := exit
	defer func() { exit = oldExit }()
	exit = func(code int) {
		exited = true
	}

	featureStartCmd.Run(featureStartCmd, []string{"fail-feature"})

	if !exited {
		t.Error("Expected exit(1) on failure")
	}
}
