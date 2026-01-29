package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveExternalCall_PartialSuffixBug(t *testing.T) {
	tmpDir := t.TempDir()

	// Create package "utils"
	dir1 := filepath.Join(tmpDir, "utils")
	os.MkdirAll(dir1, 0755)
	os.WriteFile(filepath.Join(dir1, "f.go"), []byte("package utils\nfunc F() {}"), 0644)

	// Create package "autils"
	dir2 := filepath.Join(tmpDir, "autils")
	os.MkdirAll(dir2, 0755)
	os.WriteFile(filepath.Join(dir2, "f.go"), []byte("package autils\nfunc F() {}"), 0644)

	// Main imports "my/autils" and calls F()
	// "my/autils" matches "utils" (len 5) if we only check string suffix!
	// "my/autils" matches "autils" (len 6).

	// If we import "my/autils", "autils" wins (longer). Correct.

	// BUT if we import "my/utils".
	// "my/utils" matches "utils". Correct.
	// "my/utils" does NOT match "autils". Correct.

	// What if we import "my/extra/utils"?
	// Matches "utils".

	// What if we have "pkg/utils" and "utils"?
	// Import "my/pkg/utils".
	// Matches "utils" (len 5).
	// Matches "pkg/utils" (len 9).
	// "pkg/utils" wins. Correct.

	// The ambiguity arises if the SHORTER one is a suffix of the LONGER one's PATH?
	// No.

	// The ambiguity arises if the SHORTER one is a suffix of the IMPORT, but NOT a path component.
	// e.g. "utils" matches "autils".
	// If I import "my/autils".
	// It matches "utils" (node 1) AND "autils" (node 2).
	// "autils" is longer, so it wins.

	// So where is the bug?
	// If I import "my/autils", and "autils" DOES NOT EXIST locally.
	// But "utils" exists locally.
	// Then "my/autils" matches "utils" (suffix).
	// So we resolve "my/autils" to local "utils" package!
	// THIS IS WRONG.

	// Setup:
	// Local: "utils"
	// Import: "my/autils" (which is external or missing local).
	// Expected: Should NOT resolve to "utils".

	dirMain := filepath.Join(tmpDir, "main")
	os.MkdirAll(dirMain, 0755)
	content := `package main
import "my/autils"
func Main() {
	autils.F()
}
`
	os.WriteFile(filepath.Join(dirMain, "main.go"), []byte(content), 0644)

	// We only create "utils" locally. We do NOT create "autils".
	// But wait, the test above creates both. Let's delete autils.
	os.RemoveAll(dir2)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check edge
	// Should NOT be "utils.F".
	// Should be external "my/autils.F" or empty/ambiguous.

	var target string
	for _, edge := range cg.Edges {
		if edge.From == "main.Main" {
			target = edge.To
		}
	}

	// Current behavior: "utils.F" because "my/autils" ends with "utils".
	// Expected behavior: "my/autils.F" (unresolved external) or at least NOT "utils.F".

	assert.NotEqual(t, "utils.F", target, "Should not resolve 'my/autils' to 'utils' just because of suffix match")
}
