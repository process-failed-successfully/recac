package runner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitializeLogging_RespectsRecacLogsDir(t *testing.T) {
	// Setup temporary directory for logs
	tmpDir := t.TempDir()

	// We expect the logs to be created inside this directory
	// Based on memory: "resolves the log directory by checking RECAC_LOGS_DIR first; if unset, it falls back to os.Getwd(), appending agents/logs to the resolved base path."
	// This likely means:
	// base := os.Getenv("RECAC_LOGS_DIR")
	// if base == "" { base, _ = os.Getwd() }
	// agentsLogsDir := filepath.Join(base, "agents", "logs")

	os.Setenv("RECAC_LOGS_DIR", tmpDir)
	defer os.Unsetenv("RECAC_LOGS_DIR")

	// Call the function under test
	_ = initializeLogging("test-project")

	// Check if "agents/logs" was created inside tmpDir
	expectedDir := filepath.Join(tmpDir, "agents", "logs")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Expected logs directory to be created at %s, but it does not exist", expectedDir)

		// Check if it was created in CWD (the bug)
		cwd, _ := os.Getwd()
		wrongDir := filepath.Join(cwd, "agents", "logs")
		if _, err := os.Stat(wrongDir); err == nil {
			t.Logf("Bug confirmed: Logs directory created in CWD: %s", wrongDir)
			// Cleanup the polluted directory
			// os.RemoveAll(wrongDir) // Don't delete it yet, maybe other tests use it? But we want to clean up.
            // Actually, if we are running in the repo, we should be careful.
            // But this is a reproduction test.
		}
	}
}
