package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanFile_Optimization(t *testing.T) {
	// Create a temp file
	tempDir, err := os.MkdirTemp("", "scan-opt-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	file := filepath.Join(tempDir, "test.go")
	content := []byte(`
// TODO: normal todo
// Todo: mixed case
// todo: lowercase
// NOTODO: should not match (regex handles \b)
// TODOList: should not match (regex handles \b)
//   TODO   : with spaces
// FIXME: fixme
// BUG: bug
// HACK: hack
// XXX: xxx
// ÅTODO: latin1 char before (should match if word boundary allows, but \b handles it)
// TODOÅ: latin1 char after
`)
	err = os.WriteFile(file, content, 0644)
	require.NoError(t, err)

	results, err := scanFile(file)
	require.NoError(t, err)

	// map of line number to type
	found := make(map[int]string)
	for _, r := range results {
		found[r.Line] = r.Type
	}

	// Line 2: // TODO: normal todo -> Match
	assert.Equal(t, "TODO", found[2], "Line 2 should match")
	// Line 3: // Todo: mixed case -> Match
	assert.Equal(t, "TODO", found[3], "Line 3 should match")
	// Line 4: // todo: lowercase -> Match
	assert.Equal(t, "TODO", found[4], "Line 4 should match")

	// Line 5: // NOTODO: -> Should NOT match
	assert.NotContains(t, found, 5, "Line 5 should NOT match")

	// Line 6: // TODOList: -> Should NOT match
	assert.NotContains(t, found, 6, "Line 6 should NOT match")

	// Line 7: //   TODO   : -> Match
	assert.Equal(t, "TODO", found[7], "Line 7 should match")

	// Line 8: FIXME -> Match
	assert.Equal(t, "FIXME", found[8], "Line 8 should match")

    // Line 12: ÅTODO.
    // If \b matches, it should be found.
    // If scanning works correctly, it should be found.
    assert.Equal(t, "TODO", found[12], "Line 12 (ÅTODO) should match")
}
