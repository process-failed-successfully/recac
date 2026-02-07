package runner

import (
	"os"
	"path/filepath"
	"testing"
    "log/slog"

	"github.com/stretchr/testify/assert"
)

func TestGetDBConfig(t *testing.T) {
	workspace := "/tmp/workspace"

	t.Run("Default", func(t *testing.T) {
		t.Setenv("RECAC_DB_TYPE", "")
		t.Setenv("RECAC_DB_URL", "")
		cfg := getDBConfig(workspace)
		assert.Equal(t, "sqlite", cfg.Type)
		assert.Equal(t, filepath.Join(workspace, ".recac.db"), cfg.ConnectionString)
	})

	t.Run("Postgres", func(t *testing.T) {
		t.Setenv("RECAC_DB_TYPE", "postgres")
		t.Setenv("RECAC_DB_URL", "postgres://user:pass@localhost:5432/db")

		cfg := getDBConfig(workspace)
		assert.Equal(t, "postgres", cfg.Type)
		assert.Equal(t, "postgres://user:pass@localhost:5432/db", cfg.ConnectionString)
	})

    t.Run("SQLite Explicit", func(t *testing.T) {
        t.Setenv("RECAC_DB_TYPE", "sqlite")
        t.Setenv("RECAC_DB_URL", "")

        cfg := getDBConfig(workspace)
        assert.Equal(t, "sqlite", cfg.Type)
        assert.Equal(t, filepath.Join(workspace, ".recac.db"), cfg.ConnectionString)
    })
}

func TestInitializeLogging(t *testing.T) {
    // Create temp dir
    tmpDir, err := os.MkdirTemp("", "test-logging")
    assert.NoError(t, err)
    defer os.RemoveAll(tmpDir)

    // Inject tmpDir as rootDir
    logger := initializeLogging("test-project", tmpDir)
    assert.NotNil(t, logger)
    assert.IsType(t, &slog.Logger{}, logger)

    // Check if agents/logs dir exists
    logsDir := filepath.Join(tmpDir, "agents", "logs")
    _, err = os.Stat(logsDir)
    assert.NoError(t, err, "agents/logs directory should be created")

    // Note: Log file is NOT created in tests due to race condition fix
}
