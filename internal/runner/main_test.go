package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Setup temporary logs directory for tests to avoid polluting source tree
	tmpLogs, err := os.MkdirTemp("", "recac-logs-*")
	if err == nil {
		os.Setenv("RECAC_LOGS_DIR", tmpLogs)
		defer os.RemoveAll(tmpLogs)
	}

	os.Exit(m.Run())
}
