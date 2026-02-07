package runner

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Isolate logs to a temp directory to prevent polluting the source tree and race conditions
	tmpLogs, err := os.MkdirTemp("", "recac-runner-logs-")
	if err != nil {
		panic(err)
	}
	os.Setenv("RECAC_LOGS_DIR", tmpLogs)

	code := m.Run()

	os.RemoveAll(tmpLogs)
	os.Exit(code)
}
