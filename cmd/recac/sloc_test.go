package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlocCmd(t *testing.T) {
	tempDir := t.TempDir()

	// Create a dummy Go file
	goFileContent := `package main

import "fmt"

// This is a comment
func main() {
	fmt.Println("Hello")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(goFileContent), 0644))

	// Create a dummy python file
	pyFileContent := `
def test():
    # Comment
    print("test")
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "test.py"), []byte(pyFileContent), 0644))

	// Create an ignored directory
	gitDir := filepath.Join(tempDir, ".git")
	require.NoError(t, os.Mkdir(gitDir, 0755))
	gitFileContent := `ignore me`
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "ignore.txt"), []byte(gitFileContent), 0644))

	// Create command directly and run it instead of using Execute, which parses flags
	// and might bleed states.

	cmd := slocCmd

	// Temporarily redirect output
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)

	// Cobra sets args, but if it was run previously it might have leftover states.
	// It's safer to invoke RunE directly for testing simple logic
	err := cmd.RunE(cmd, []string{tempDir})
	require.NoError(t, err)

	output := stdout.String()

	// Assertions based on tabwriter formatting
	assert.Contains(t, output, ".go")
	assert.Contains(t, output, ".py")
	assert.NotContains(t, output, "ignore.txt")
	assert.NotContains(t, output, ".git")

	// Split and check strings to avoid tab issues
	assert.Contains(t, output, "8") // Total Go lines
	assert.Contains(t, output, "5") // Code Go lines
}
