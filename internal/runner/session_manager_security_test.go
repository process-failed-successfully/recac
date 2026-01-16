package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSessionManager_Security_PathTraversal(t *testing.T) {
	// Setup temporary home directory
	tmpDir, err := os.MkdirTemp("", "recac-test-security")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create manager
	sm, err := NewSessionManagerWithDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	// Prepare a dummy executable
	lsCmd, err := exec.LookPath("ls")
	if err != nil {
		t.Skip("ls command not found, skipping test")
	}

	tests := []struct {
		name        string
		sessionName string
	}{
		{"Parent Directory", "../evil"},
		{"Root Directory", "/tmp/evil"},
		{"Nested Directory", "a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command := []string{lsCmd}
			_, err := sm.StartSession(tt.sessionName, "test goal", command, tmpDir)
			if err == nil {
				t.Errorf("StartSession should fail for name '%s', but it succeeded", tt.sessionName)

				// Clean up if it actually created files (to be nice)
				// We don't know where it created them exactly without recalculating logic,
				// but for "Parent Directory" it would be in tmpDir/../evil.json
				if tt.sessionName == "../evil" {
					path := filepath.Join(tmpDir, "../evil.json")
					os.Remove(path)
					path = filepath.Join(tmpDir, "../evil.log")
					os.Remove(path)
				}
			}
		})
	}
}

func TestSessionManager_Security_LoadSession(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "recac-test-security-load")
	defer os.RemoveAll(tmpDir)
	sm, _ := NewSessionManagerWithDir(tmpDir)

	_, err := sm.LoadSession("../outside")
	if err == nil {
		t.Error("LoadSession should fail for path traversal name")
	}
}
