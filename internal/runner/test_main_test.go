package runner

import (
	"os"
	"testing"
)

// TestMain sets up the test environment for the runner package
func TestMain(m *testing.M) {
	// Create a temporary directory for logs to prevent pollution
	tmpDir, err := os.MkdirTemp("", "recac-runner-tests")
	if err != nil {
		panic(err)
	}

	// Set the environment variable for the session logger
	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Run tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)

	os.Exit(code)
}
