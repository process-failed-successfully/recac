package runner

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup temporary log directory for tests
	tmpDir, err := os.MkdirTemp("", "recac-runner-logs-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create temp log dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("RECAC_LOGS_DIR", tmpDir)

	os.Exit(m.Run())
}
