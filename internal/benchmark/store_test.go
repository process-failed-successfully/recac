package benchmark

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileStore(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "benchmarks.json")
	store, err := NewFileStore(path)
	assert.NoError(t, err)

	// Test LoadAll on empty
	runs, err := store.LoadAll()
	assert.NoError(t, err)
	assert.Empty(t, runs)

	// Test Save
	run1 := Run{
		Timestamp: time.Now().Add(-1 * time.Hour),
		Commit:    "abc",
		Results: []Result{
			{Name: "B1", NsPerOp: 100},
		},
	}
	err = store.Save(run1)
	assert.NoError(t, err)

	// Test LoadLatest
	latest, err := store.LoadLatest()
	assert.NoError(t, err)
	assert.Equal(t, "abc", latest.Commit)

	// Test Save second run
	run2 := Run{
		Timestamp: time.Now(),
		Commit:    "def",
		Results: []Result{
			{Name: "B1", NsPerOp: 110},
		},
	}
	err = store.Save(run2)
	assert.NoError(t, err)

	// Verify persistence and order
	runs, err = store.LoadAll()
	assert.NoError(t, err)
	assert.Len(t, runs, 2)
	assert.Equal(t, "abc", runs[0].Commit)
	assert.Equal(t, "def", runs[1].Commit)
}
