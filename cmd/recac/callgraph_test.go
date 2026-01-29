package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create main.go with a method call
	// pkg.(Type).Method -> complex ID
	mainContent := `package main
type Service struct{}
func (s *Service) DoWork() {}
func main() {
	s := &Service{}
	s.DoWork()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Set the global flag to point to our temp dir
	// We need to use the variable defined in callgraph.go
	oldDir := callGraphDir
	callGraphDir = tmpDir
	defer func() { callGraphDir = oldDir }()

	// Capture output
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Run
	err = runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := out.String()

	// Verify Output
	assert.Contains(t, output, "graph LR")

	// Check for sanitized IDs
	// ID for main.go: "main.(Service).DoWork" -> "main__Service__DoWork"
	// Note: sanitizeMermaidID replaces '.' with '_' and '(' / ')' with '_'

	assert.Contains(t, output, "main__Service__DoWork")

	// Ensure no raw parentheses remain in IDs or edges
	// Note: Labels might contain them if we didn't sanitize labels,
	// but generateMermaidCallGraph sets label = parts[len(parts)-1].
	// label for "main.(Service).DoWork" -> "DoWork" (if split by /? no, split by /)
	// Wait, internal/analysis logic for ID: "main.(Service).DoWork"
	// generateMermaidCallGraph: parts := strings.Split(label, "/")
	// label = parts[len(parts)-1] -> "main.(Service).DoWork"

	// So the Label in Mermaid WILL contain parentheses: ["main.(Service).DoWork"]
	// Labels in Mermaid are inside quotes ["..."], so parentheses ARE allowed there.
	// But IDs must be sanitized.

	// So we verify that the DEFINITION uses sanitized ID:
	// main__Service__DoWork["main.(Service).DoWork"]

	expectedNode := `main__Service__DoWork["main.(Service).DoWork"]`
	assert.Contains(t, output, expectedNode)
}
