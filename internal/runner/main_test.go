package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Create a temporary directory for logs to prevent pollution
	tmpDir, err := os.MkdirTemp("", "recac-logs-test-")
	if err != nil {
		panic(err)
	}

	// Set the environment variable so initializeLogging uses it
	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Run tests
	code := m.Run()

	// Cleanup (must be explicit because os.Exit bypasses defers)
	os.RemoveAll(tmpDir)

	// Exit with the code
	os.Exit(code)
}
