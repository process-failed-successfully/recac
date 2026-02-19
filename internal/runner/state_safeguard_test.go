package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type TestState struct {
	Value string `json:"value"`
}

func TestLoadSafeguardedState(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-safeguard-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	t.Run("ValidFile", func(t *testing.T) {
		path := filepath.Join(tmpDir, "valid.json")
		state := TestState{Value: "test"}
		data, _ := json.Marshal(state)
		err := os.WriteFile(path, data, 0600)
		require.NoError(t, err)

		var loaded TestState
		err = LoadSafeguardedState(path, &loaded)
		assert.NoError(t, err)
		assert.Equal(t, "test", loaded.Value)
	})

	t.Run("MissingFile", func(t *testing.T) {
		path := filepath.Join(tmpDir, "missing.json")
		var loaded TestState
		err := LoadSafeguardedState(path, &loaded)
		assert.NoError(t, err)
		assert.Equal(t, "", loaded.Value)
	})

	t.Run("CorruptFile", func(t *testing.T) {
		path := filepath.Join(tmpDir, "corrupt.json")
		err := os.WriteFile(path, []byte("{invalid-json"), 0600)
		require.NoError(t, err)

		var loaded TestState
		err = LoadSafeguardedState(path, &loaded)
		assert.NoError(t, err) // Should return nil and delete file
		assert.Equal(t, "", loaded.Value)

		// Verify file is deleted
		_, err = os.Stat(path)
		assert.True(t, os.IsNotExist(err), "Corrupt file should be deleted")
	})

	t.Run("PermissionError", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("Skipping permission test as root")
		}
		// Create a directory with no permissions
		lockedDir := filepath.Join(tmpDir, "locked")
		err := os.Mkdir(lockedDir, 0000)
		require.NoError(t, err)
		defer os.Chmod(lockedDir, 0700)

		path := filepath.Join(lockedDir, "file.json")
		var loaded TestState
		err = LoadSafeguardedState(path, &loaded)
		assert.Error(t, err)
		// Error message depends on OS
	})
}
