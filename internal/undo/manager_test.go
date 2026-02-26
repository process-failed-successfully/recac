package undo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_CaptureAndRestore(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// 1. Setup initial state (Modify scenario)
	existingFile := filepath.Join(tmpDir, "existing.txt")
	err := os.WriteFile(existingFile, []byte("original content"), 0644)
	require.NoError(t, err)

	// 2. Setup initial state (Create scenario - file doesn't exist yet)
	newFile := filepath.Join(tmpDir, "new.txt")

	// 3. Capture
	opID, err := m.Capture(existingFile, newFile)
	require.NoError(t, err)
	assert.NotEmpty(t, opID)

	// Verify backup created for existing file
	ops, err := m.List()
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Equal(t, opID, ops[0].ID)
	assert.Len(t, ops[0].Changes, 2)

	// 4. Perform modifications
	err = os.WriteFile(existingFile, []byte("modified content"), 0644)
	require.NoError(t, err)

	err = os.WriteFile(newFile, []byte("created content"), 0644)
	require.NoError(t, err)

	// 5. Restore
	err = m.Restore(opID)
	require.NoError(t, err)

	// 6. Verify restoration
	// Existing file should be reverted
	content, err := os.ReadFile(existingFile)
	require.NoError(t, err)
	assert.Equal(t, "original content", string(content))

	// New file should be deleted
	_, err = os.Stat(newFile)
	assert.True(t, os.IsNotExist(err), "new file should be deleted")

	// History should be cleared
	ops, err = m.List()
	require.NoError(t, err)
	assert.Empty(t, ops)
}

func TestManager_List(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create fake history
	op1 := Operation{ID: "1", Timestamp: time.Now().Add(-1 * time.Hour)}
	op2 := Operation{ID: "2", Timestamp: time.Now()}

	// Save manually to test List sorting
	m.saveHistory([]Operation{op1, op2})

	ops, err := m.List()
	require.NoError(t, err)
	require.Len(t, ops, 2)
	assert.Equal(t, "2", ops[0].ID, "Should be sorted recent first")
	assert.Equal(t, "1", ops[1].ID)
}
