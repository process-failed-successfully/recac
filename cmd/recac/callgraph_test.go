package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()

	// Create some sample code
	mainContent := `package main
import "fmt"
func main() {
    helper()
    fmt.Println("Done")
}
func helper() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	t.Run("Generate full graph", func(t *testing.T) {
		cmd := NewCallGraphCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--dir", tmpDir})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "graph LR")
		// The ID format is relative path + package + func
		// If tmpDir is root, package is main.
		// ID: main.main -> main_main
		assert.Contains(t, output, "main_main")
		assert.Contains(t, output, "main_helper")
		assert.Contains(t, output, "main_main --> main_helper")
	})

	t.Run("Focus filter", func(t *testing.T) {
		cmd := NewCallGraphCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--dir", tmpDir, "--focus", "helper"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "main_helper")
		assert.Contains(t, output, "main_main --> main_helper")
	})

    t.Run("Focus filter with no matches", func(t *testing.T) {
		cmd := NewCallGraphCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--dir", tmpDir, "--focus", "nonexistent"})

		err := cmd.Execute()
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "graph LR")
        // Should not contain nodes
		assert.NotContains(t, output, "main_main")
		assert.NotContains(t, output, "main_helper")
	})

    t.Run("Sanitization check", func(t *testing.T) {
        // Create code with special chars in types/methods
        // e.g. generic function or method on struct
        content := `package pkg
type MyStruct struct{}
func (s *MyStruct) Method() {}
func Call() {
    s := &MyStruct{}
    s.Method()
}
`
        pkgDir := filepath.Join(tmpDir, "pkg")
        os.Mkdir(pkgDir, 0755)
        os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte(content), 0644)

        cmd := NewCallGraphCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs([]string{"--dir", tmpDir})

		err := cmd.Execute()
		require.NoError(t, err)

        output := buf.String()
        // ID: pkg.(MyStruct).Method -> pkg__MyStruct__Method
        assert.Contains(t, output, "pkg__MyStruct__Method")
    })
}
