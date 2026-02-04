package runner

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/security"
	"recac/internal/telemetry"
	"testing"
)

// TestSecurity_DirectoryEscape verifies that a local agent cannot write files outside its workspace.
// This simulates a directory traversal attack.
func TestSecurity_DirectoryEscape(t *testing.T) {
	// 1. Setup secure workspace and an external "target" directory
	tmpDir := t.TempDir()
	workspaceDir := filepath.Join(tmpDir, "workspace")
	externalDir := filepath.Join(tmpDir, "external")

	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	if err := os.MkdirAll(externalDir, 0755); err != nil {
		t.Fatalf("Failed to create external dir: %v", err)
	}

	// Target file that shouldn't be created
	hackedFile := filepath.Join(externalDir, "hacked.txt")

	// 2. Initialize Session in Local Agent mode
	// We use a nil Docker client because we are running locally
	logger := telemetry.NewLogger(true, "", false)

	session := &Session{
		Workspace:     workspaceDir,
		UseLocalAgent: true,
		Project:       "security-test",
		Logger:        logger,
		// Real scanner to test if it blocks the attack
		Scanner:       security.NewRegexScanner(),
	}

	// The attack payload: cd .. && cd external && touch hacked.txt
	// We need to calculate relative path to external from workspace
	relPath, _ := filepath.Rel(workspaceDir, externalDir)
	attackCmd := "cd " + relPath + " && touch hacked.txt"

	response := "```bash\n" + attackCmd + "\n```"

	// 3. Execute
	_, err := session.ProcessResponse(context.Background(), response)
	if err != nil {
		// It might fail if commands fail, but we want to check side effects
		t.Logf("ProcessResponse error (might be expected): %v", err)
	}

	// 4. Verify Vulnerability
	if _, err := os.Stat(hackedFile); err == nil {
		t.Errorf("CRITICAL SECURITY VULNERABILITY: Agent successfully created file outside workspace at %s", hackedFile)
	} else {
		t.Log("Attack failed (File not created). Validating why...")
	}
}
