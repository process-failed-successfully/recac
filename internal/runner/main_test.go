package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Create a temporary directory for logs to avoid polluting the source tree
	tmpDir, err := os.MkdirTemp("", "recac-logs-runner-*")
	if err != nil {
		panic(err)
	}

	// Set RECAC_LOGS_DIR env var
	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Run tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)

	os.Exit(code)
}
