package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequenceAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	mainGo := `package main
func main() {
    Foo()
}
func Foo() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644)
	require.NoError(t, err)

	out, err := analysis.GenerateSequence(tmpDir, "main", 5)
	require.NoError(t, err)
	assert.Contains(t, out, "sequenceDiagram")
	assert.Contains(t, out, "participant main as main")
	assert.Contains(t, out, "main->>main: Foo()")
}

func TestSequenceCmd(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()

	// main.go
	mainGo := `package main
func main() {
    Foo()
}

func Foo() {
    Bar()
    s := Struct{}
    s.Method()
}

func Bar() {
    // do something
}

type Struct struct{}
func (s *Struct) Method() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644)
	require.NoError(t, err)

	// Execute command via rootCmd
	buf := new(bytes.Buffer)
	cmd := rootCmd
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Reset flags
	sequenceDepth = 5
	sequenceOutput = ""
	sequenceDir = "." // Reset to default to ensure flag parses it

	// Pass arguments: subcommand + args
	cmd.SetArgs([]string{"sequence", "main", "--dir", tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	// Debug output if empty
	if output == "" {
		t.Logf("Output was empty. sequenceDir=%s", sequenceDir)
	}

	assert.Contains(t, output, "sequenceDiagram")
	assert.Contains(t, output, "participant main as main")
	assert.Contains(t, output, "main->>main: Foo()")
	assert.Contains(t, output, "main->>main: Bar()")
}
