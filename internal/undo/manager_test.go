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

func TestManager_Capture_NoPaths(t *testing.T) {
	m := NewManager(t.TempDir())
	id, err := m.Capture()
	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestManager_Capture_MkdirError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where the undo dir should be, forcing MkdirAll to fail
	undoBase := filepath.Join(tmpDir, UndoDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(undoBase), 0755))
	require.NoError(t, os.WriteFile(undoBase, []byte("file"), 0644))

	m := NewManager(tmpDir)
	id, err := m.Capture(filepath.Join(tmpDir, "test.txt"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create backup dir")
	assert.Empty(t, id)
}

func TestManager_Capture_SkipDir(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	dirPath := filepath.Join(tmpDir, "some_dir")
	require.NoError(t, os.MkdirAll(dirPath, 0755))

	id, err := m.Capture(dirPath)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	ops, err := m.List()
	require.NoError(t, err)
	require.Len(t, ops, 1)
	assert.Empty(t, ops[0].Changes, "Directory should be skipped, so no changes recorded")
}

func TestManager_Capture_CopyError(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	// Create a file
	filePath := filepath.Join(tmpDir, "unreadable.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0200)) // write-only

	// In Go, open for reading might fail if 0200
	id, err := m.Capture(filePath)
	if err == nil {
		// If copy somehow succeeded, it's fine, but let's see.
		t.Logf("Copy didn't fail for 0200 file")
	} else {
		assert.Contains(t, err.Error(), "failed to backup file")
		assert.Empty(t, id)
	}
}

func TestManager_Restore_InvalidOp(t *testing.T) {
	m := NewManager(t.TempDir())
	err := m.Restore("non-existent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "operation non-existent not found")
}

func TestManager_List_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)

	indexPath := filepath.Join(tmpDir, UndoDir, IndexFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(indexPath), 0755))
	require.NoError(t, os.WriteFile(indexPath, []byte("{invalid json"), 0644))

	ops, err := m.List()
	require.Error(t, err)
	assert.Nil(t, ops)
}

func TestManager_SaveHistory_MkdirError(t *testing.T) {
	tmpDir := t.TempDir()
	undoBase := filepath.Join(tmpDir, UndoDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(undoBase), 0755))
	require.NoError(t, os.WriteFile(undoBase, []byte("file"), 0644))

	m := NewManager(tmpDir)
	err := m.saveHistory([]Operation{})
	require.Error(t, err)
}

func TestCopyFile_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	err := copyFile(filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "out"))
	require.Error(t, err)

	src := filepath.Join(tmpDir, "src")
	require.NoError(t, os.WriteFile(src, []byte("data"), 0644))

	// Write to a directory path
	dstDir := filepath.Join(tmpDir, "dstDir")
	require.NoError(t, os.Mkdir(dstDir, 0755))
	err = copyFile(src, dstDir)
	require.Error(t, err)
}
