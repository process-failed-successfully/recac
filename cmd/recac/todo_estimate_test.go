package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"recac/internal/agent"
	"github.com/stretchr/testify/assert"
)

// EstimateMockAgent is a simple mock that returns a fixed response
type EstimateMockAgent struct {
	Response string
}

func (m *EstimateMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *EstimateMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestTodoEstimateCmd(t *testing.T) {
	// 1. Setup temporary directory
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	// 2. Create dummy files
	todoContent := "- [ ] [main.go:5] TODO: Fix the flux capacitor\n"
	if err := os.WriteFile("TODO.md", []byte(todoContent), 0644); err != nil {
		t.Fatalf("failed to write TODO.md: %v", err)
	}

	mainContent := `package main

func main() {
	// TODO: Fix the flux capacitor
	println("Hello World")
}
`
	if err := os.WriteFile("main.go", []byte(mainContent), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// 3. Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	expectedResponse := "Complexity: Low\nPlan: Just fix it."
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &EstimateMockAgent{Response: expectedResponse}, nil
	}

	// 4. Run Command
	// Capture output
	outputBuf := new(bytes.Buffer)
	todoEstimateCmd.SetOut(outputBuf)
	todoEstimateCmd.SetErr(outputBuf)

	// Execute directly via RunE to avoid full cobra dispatch complexity in test
	err := todoEstimateCmd.RunE(todoEstimateCmd, []string{"1"})
	assert.NoError(t, err)

	// Verify Output
	output := outputBuf.String()
	assert.Contains(t, output, "Estimating TODO in main.go at line 5")
	assert.Contains(t, output, expectedResponse)
}
