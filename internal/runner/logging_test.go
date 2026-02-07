package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeLogging_WithRecacLogsDir(t *testing.T) {
	// Create a temporary directory to simulate RECAC_LOGS_DIR
	tmpDir := t.TempDir()

	// Set the environment variable
	t.Setenv("RECAC_LOGS_DIR", tmpDir)

	// We can't call initializeLogging directly as it's unexported.
	// But we can call NewSessionWithConfig which calls it.
	// We pass empty strings for most args as we only care about logging setup side effects.
	NewSessionWithConfig(t.TempDir(), "test-project-repro", "mock", "mock", nil)

	// Check if log file was created in tmpDir
	// We expect a file matching pattern "test-project-repro_agent_test-project-repro_*.log"
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read tmpDir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if !entry.IsDir() {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected log file to be created in RECAC_LOGS_DIR (%s), but it was empty.", tmpDir)

		// Check if it was created in CWD instead (the bug)
		cwd, _ := os.Getwd()
		defaultLogsDir := filepath.Join(cwd, "agents", "logs")
		entriesCwd, err := os.ReadDir(defaultLogsDir)
		if err == nil {
			for _, entry := range entriesCwd {
				if !entry.IsDir() { // simplistic check
					t.Logf("Found log file in CWD: %s", entry.Name())
				}
			}
		}
	}
}
