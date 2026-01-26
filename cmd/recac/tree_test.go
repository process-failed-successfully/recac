package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTreeCmd(t *testing.T) {
	// Create a temp dir
	tmpDir, err := os.MkdirTemp("", "recac-tree-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some files
	// 1. main.go - with complexity and TODO
	mainContent := `package main
func main() {
	if true {
		if true {
			// TODO: Fix nested if
		}
	}
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. README.md
	err = os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Hello"), 0644)
	require.NoError(t, err)

	// 3. Subdirectory
	subDir := filepath.Join(tmpDir, "pkg")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// 4. pkg/helper.go - simple
	helperContent := `package pkg
func Help() {}
`
	err = os.WriteFile(filepath.Join(subDir, "helper.go"), []byte(helperContent), 0644)
	require.NoError(t, err)

	// Set mod times to something deterministic if possible, or just check format.
	// We can't easily change mod time in a way that race conditions won't affect "now" vs "1s ago".
	// But we can check that output contains "now" or "ago".

	// Ensure file times are slightly in the past so they show up as "now" or "1m ago" but valid.
	now := time.Now()
	os.Chtimes(filepath.Join(tmpDir, "main.go"), now, now)

	// Run tree command
	output, err := executeCommand(rootCmd, "tree", tmpDir)
	require.NoError(t, err)

	// Verify output
	// Should contain:
	// ├── README.md
	// ├── main.go [70B | now | C:3 | T:1]
	// └── pkg
	//     └── helper.go

	assert.Contains(t, output, "README.md")
	assert.Contains(t, output, "main.go")
	assert.Contains(t, output, "pkg")
	assert.Contains(t, output, "helper.go")

	// Check for metadata
	// We can't predict exact size easily (string len), but we know content length.
	// main.go ~ 70 bytes.
	assert.Contains(t, output, "C:3") // Complexity of nested ifs
	assert.Contains(t, output, "T:1") // TODO count

	// Test --only-go flag
	outputGo, err := executeCommand(rootCmd, "tree", tmpDir, "--only-go")
	require.NoError(t, err)
	assert.NotContains(t, outputGo, "README.md")
	assert.Contains(t, outputGo, "main.go")
	assert.Contains(t, outputGo, "helper.go")

	// Test --depth flag
	outputDepth, err := executeCommand(rootCmd, "tree", tmpDir, "--depth=0")
	require.NoError(t, err)
	assert.Contains(t, outputDepth, "main.go")
	assert.NotContains(t, outputDepth, "helper.go") // Should be hidden
}

func TestTreeCmd_Sort(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-tree-sort-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Large file
	err = os.WriteFile(filepath.Join(tmpDir, "large.txt"), make([]byte, 1000), 0644)
	require.NoError(t, err)

	// Small file
	err = os.WriteFile(filepath.Join(tmpDir, "small.txt"), []byte("a"), 0644)
	require.NoError(t, err)

	// Sort by size
	output, err := executeCommand(rootCmd, "tree", tmpDir, "--sort=size")
	require.NoError(t, err)

	// We expect large.txt to come before small.txt (descending size)
	// But directories usually come first. Here only files.
	// Verify order in output string
	idxLarge := strings.Index(output, "large.txt")
	idxSmall := strings.Index(output, "small.txt")
	assert.Less(t, idxLarge, idxSmall, "large.txt should appear before small.txt when sorted by size")
}
