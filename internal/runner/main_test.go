package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Create a temporary directory for logs
	tmpDir, err := os.MkdirTemp("", "recac-logs-runner-test")
	if err != nil {
		panic(err)
	}
	// We can't defer remove because os.Exit stops defers.
	// But we can remove it manually after Run.

	// Set RECAC_LOGS_DIR to the temporary directory
	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Run tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)
	os.Unsetenv("RECAC_LOGS_DIR")

	os.Exit(code)
}
