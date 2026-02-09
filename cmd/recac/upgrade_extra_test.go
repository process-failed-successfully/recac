package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestCheckUpdatesNpm(t *testing.T) {
	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "npm" && args[0] == "outdated" {
			cmd := exec.Command(os.Args[0], "-test.run=TestHelperNpmOutdated", "--")
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
			return cmd
		}
		return exec.Command(name, args...)
	}

	updates, err := checkUpdatesNpm()
	if err != nil {
		t.Fatalf("checkUpdatesNpm failed: %v", err)
	}

	if len(updates) != 1 {
		t.Errorf("Expected 1 update, got %d", len(updates))
	}
	if updates[0].Name != "react" {
		t.Errorf("Expected react, got %s", updates[0].Name)
	}
}

func TestHelperNpmOutdated(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Print JSON to stdout
	json := `{
		"react": {
			"current": "17.0.0",
			"latest": "18.0.0"
		}
	}`
	os.Stdout.WriteString(json)
	os.Exit(0)
}
