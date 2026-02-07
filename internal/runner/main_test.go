package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Create a temporary directory for logs
	tmpDir, err := os.MkdirTemp("", "recac-runner-logs")
	if err != nil {
		panic(err)
	}

	// Set the environment variable
	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	// Run the tests
	code := m.Run()

	// Cleanup
	os.RemoveAll(tmpDir)

	// Exit with the code
	os.Exit(code)
}
