package main

import (
	"path/filepath"
	"recac/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSignalCommand_Privileged(t *testing.T) {
	// Setup DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create DB with empty schema/tables if needed, but sqlite auto-creates
	// We need to ensure table `signals` exists. NewStore usually handles migrations.
	config := db.StoreConfig{
		Type:             "sqlite",
		ConnectionString: dbPath,
	}

	// Initialize DB
	store, err := db.NewStore(config)
	require.NoError(t, err)
	defer store.Close()

	// Helper to run main logic
	runCmd := func(args []string) error {
		return run(args, config, "test-project")
	}

	t.Run("Non-privileged signal success", func(t *testing.T) {
		err := runCmd([]string{"agent-bridge", "signal", "foo", "bar"})
		assert.NoError(t, err)
		val, err := store.GetSignal("test-project", "foo")
		assert.NoError(t, err)
		assert.Equal(t, "bar", val)
	})

	t.Run("Privileged signal fail without flag", func(t *testing.T) {
		err := runCmd([]string{"agent-bridge", "signal", "PROJECT_SIGNED_OFF", "true"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "privileged and cannot be set via agent-bridge without --privileged flag")
	})

	t.Run("Privileged signal success with flag", func(t *testing.T) {
		err := runCmd([]string{"agent-bridge", "signal", "--privileged", "PROJECT_SIGNED_OFF", "true"})
		assert.NoError(t, err)
		val, err := store.GetSignal("test-project", "PROJECT_SIGNED_OFF")
		assert.NoError(t, err)
		assert.Equal(t, "true", val)
	})

	t.Run("Privileged signal success with flag (other privileged key)", func(t *testing.T) {
		err := runCmd([]string{"agent-bridge", "signal", "--privileged", "TRIGGER_QA", "true"})
		assert.NoError(t, err)
		val, err := store.GetSignal("test-project", "TRIGGER_QA")
		assert.NoError(t, err)
		assert.Equal(t, "true", val)
	})
}
