package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Set up temporary logs directory for tests
	tmpDir, err := os.MkdirTemp("", "recac-logs")
	if err != nil {
		panic(err)
	}

	// Set environment variable
	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Run tests
	exitVal := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)
	os.Unsetenv("RECAC_LOGS_DIR")

	os.Exit(exitVal)
}
