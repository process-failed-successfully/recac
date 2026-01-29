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
	// Setup temp dir with some code
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
import "fmt"
func main() { fmt.Println("Hello") }
`), 0644)
	require.NoError(t, err)

	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute with --dir
	cmd.SetArgs([]string{"--dir", tmpDir})
	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main.main")
}

func TestCallGraphCmd_Focus(t *testing.T) {
	// Setup temp dir with some code
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
func A() { B() }
func B() {}
`), 0644)
	require.NoError(t, err)

	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute with --dir and --focus
	cmd.SetArgs([]string{"--dir", tmpDir, "--focus", "A"})
	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main.A")
	// B should be included because A calls B
	assert.Contains(t, output, "main.B")
}

func TestCallGraphCmd_DefaultDir(t *testing.T) {
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
func main() {}
`), 0644)
	require.NoError(t, err)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	require.NoError(t, os.Chdir(tmpDir))

	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute without args (defaults to .)
	cmd.SetArgs([]string{})
	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main.main")
}
