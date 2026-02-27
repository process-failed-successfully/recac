package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// WhatIfMockAgent mocks the agent for whatif command tests
type WhatIfMockAgent struct {
	Response string
}

func (m *WhatIfMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Verify prompt contains expected context
	if !strings.Contains(prompt, "HYPOTHETICAL change") {
		return "Invalid prompt", nil
	}
	return m.Response, nil
}

func (m *WhatIfMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestWhatIfCmd(t *testing.T) {
	// 1. Setup Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := `### Impact Analysis
**Affected Components**:
- internal/db/user.go
- internal/api/handlers.go

**Breaking Changes**:
- API contract for User will change.

**Migration Steps**:
1. Run DB migration.
2. Update API structs.
`

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &WhatIfMockAgent{Response: mockResponse}, nil
	}

	// 2. Setup GenerateContextFunc Mock
	originalContextFunc := generateContextFunc
	defer func() { generateContextFunc = originalContextFunc }()

	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Codebase Context", nil
	}

	// 3. Setup Temp Dir for focus test
	tempDir, err := os.MkdirTemp("", "whatif-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a dummy file to focus on
	focusFile := filepath.Join(tempDir, "user.go")
	err = os.WriteFile(focusFile, []byte("package main\ntype User struct { ID int }"), 0644)
	assert.NoError(t, err)

	// 4. Test Case 1: Run with valid input
	cmd := &cobra.Command{}
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)

	// Reset global vars if needed (whatIfFocus is global in main package)
	// We need to inject the command logic or use the real command if possible.
	// Since `whatIfCmd` is global, we should be careful.
	// Best practice is to use the RunE directly or create a new command instance if `NewWhatIfCmd` existed.
	// `whatIfCmd` is defined in `whatif.go`. We can use it but need to reset flags.

	// Reset flag
	whatIfFocus = "." // Reset to default

	// Execute RunE directly to avoid global flag pollution if possible,
	// but flags are parsed by Cobra before RunE.
	// So we need to execute the command via Cobra's Execute or manually call RunE with context.

	// Let's call runWhatIf directly, but we need to set the context manually.
	// And we need to ensure whatIfFocus is set correctly.
	// Since `runWhatIf` uses the global `whatIfFocus` variable which is bound to the flag,
	// we can just set the variable manually for the test.

	whatIfFocus = tempDir
	err = runWhatIf(cmd, []string{"Change User.ID to string"})
	assert.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, "Simulating change impact")
	assert.Contains(t, output, "Impact Analysis")
	assert.Contains(t, output, "Affected Components")

	// 5. Test Case 2: Invalid focus path
	whatIfFocus = "/non/existent/path"
	err = runWhatIf(cmd, []string{"Change User.ID"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "focus path does not exist")
}
