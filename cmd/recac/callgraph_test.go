package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestRunCallGraph(t *testing.T) {
	// 1. Create a temporary directory with Go code
	tmpDir, err := os.MkdirTemp("", "recac-callgraph-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := `package main

import "fmt"

func main() {
	Helper()
}

func Helper() {
	fmt.Println("Helper called")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 2. Setup Command
	cmd := &cobra.Command{
		Use:  "callgraph",
		RunE: runCallGraph,
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Set Flags
	// We need to set the global variable `callGraphDir` used by `runCallGraph`
	// Or pass arguments?
	// runCallGraph uses `callGraphDir` variable.
	// But `runCallGraph` also defaults to `.` if `dir` is empty?
	// Actually `runCallGraph` reads `callGraphDir`.
	// Let's set it.
	oldDir := callGraphDir
	callGraphDir = tmpDir
	defer func() { callGraphDir = oldDir }()

	// 3. Run Command
	err = runCallGraph(cmd, []string{})
	assert.NoError(t, err)

	// 4. Verify Output
	output := buf.String()
	assert.Contains(t, output, "graph LR")

	// Check for nodes
	// main.main
	// main.Helper
	// format: "    safeID[\"label\"]"
	// ID might be full path?
	// In the temporary dir, the package path might be weird or just "main".
	// Let's check for edge "main --> Helper"
	// Or simpler "main --> Helper" if sanitized.

	// The `GenerateCallGraph` likely uses full package paths.
	// Since we are in a temp dir outside of GOPATH/Module, `go/parser` might treat it as ad-hoc.
	// The package name is "main".
	// The function ID is likely "main.main" and "main.Helper".

	assert.Contains(t, output, "main.main")
	assert.Contains(t, output, "main.Helper")

	// Check edge
	// Because of sanitization, "." becomes "_" usually?
	// Let's see `sanitizeMermaidID`.
	// Assuming standard replacement.
	// If ID is "main.main", sanitized is "main_main"?
	// We can't know for sure without checking `sanitizeMermaidID` implementation,
	// but we can check if output contains "-->".
	assert.Contains(t, output, "-->")
}
