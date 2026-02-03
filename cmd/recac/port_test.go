package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PortTestMockAgent implements agent.Agent interface
type PortTestMockAgent struct {
	Response string
}

func (m *PortTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *PortTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestPortCmd(t *testing.T) {
	// Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-port-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Mock Agent Factory
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	// Test Case 1: Missing --to flag
	t.Run("MissingToFlag", func(t *testing.T) {
		cmd, _, _ := newRootCmd() // Ensure we get fresh command structure
		// Because cobra commands are globals in this package, we need to be careful.
		// portCmd is a global variable.
		// We reset flags before use.
		portCmd.Flags().Set("to", "")
		portCmd.Flags().Set("output", "")

		inputFile := filepath.Join(tmpDir, "source.py")
		os.WriteFile(inputFile, []byte("print('hello')"), 0644)

		_, err := executeCommand(cmd, "port", inputFile)
		// Cobra usually prints usage and returns error on missing required flag if ParseFlags fails.
		// But executeCommand captures it.
		// Wait, if it's required flag, executeCommand might panic with "exit-1" or return error.
		// Our executeCommand handles panic.

		// Note: MarkFlagRequired is checked during execution.
		assert.Error(t, err)
		// The error message comes in the error object, not necessarily in stdout/stderr depending on cobra config
		assert.Contains(t, err.Error(), "required flag(s) \"to\" not set")
	})

	// Test Case 2: Input file missing
	t.Run("InputFileMissing", func(t *testing.T) {
		cmd, _, _ := newRootCmd()
		portCmd.Flags().Set("to", "go")

		missingFile := filepath.Join(tmpDir, "nonexistent.py")

		_, err := executeCommand(cmd, "port", missingFile, "--to", "go")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read input file")
	})

	// Test Case 3: Successful Port
	t.Run("Success", func(t *testing.T) {
		inputFile := filepath.Join(tmpDir, "hello.py")
		err := os.WriteFile(inputFile, []byte("print('hello world')"), 0644)
		require.NoError(t, err)

		outputFile := filepath.Join(tmpDir, "hello.go")

		// Mock Agent
		mockAgent := &PortTestMockAgent{
			Response: "```go\npackage main\nimport \"fmt\"\nfunc main() {\n\tfmt.Println(\"hello world\")\n}\n```",
		}
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		cmd, _, _ := newRootCmd()
		portCmd.Flags().Set("to", "go")
		portCmd.Flags().Set("output", outputFile)

		out, err := executeCommand(cmd, "port", inputFile, "--to", "go", "--output", outputFile)
		assert.NoError(t, err)
		assert.Contains(t, out, "Ported code written to")

		// Verify output file content
		content, err := os.ReadFile(outputFile)
		assert.NoError(t, err)

		expected := `package main
import "fmt"
func main() {
	fmt.Println("hello world")
}`
		// Trim space for comparison to handle newline differences
		assert.Equal(t, strings.TrimSpace(expected), strings.TrimSpace(string(content)))
	})

	// Test Case 4: Stdout output
	t.Run("Stdout", func(t *testing.T) {
		inputFile := filepath.Join(tmpDir, "math.js")
		err := os.WriteFile(inputFile, []byte("const add = (a, b) => a + b;"), 0644)
		require.NoError(t, err)

		mockAgent := &PortTestMockAgent{
			Response: "def add(a, b):\n    return a + b",
		}
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		cmd, _, _ := newRootCmd()
		// Reset output flag
		portCmd.Flags().Set("output", "")

		out, err := executeCommand(cmd, "port", inputFile, "--to", "python")
		assert.NoError(t, err)

		assert.Contains(t, out, "def add(a, b):")
		assert.Contains(t, out, "return a + b")
	})
}
