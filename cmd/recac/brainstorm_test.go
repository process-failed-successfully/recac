package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAgentForBrainstorm implements agent.Agent interface
type MockAgentForBrainstorm struct {
	Prompts []string
}

func (m *MockAgentForBrainstorm) Send(ctx context.Context, prompt string) (string, error) {
	m.Prompts = append(m.Prompts, prompt)

	// Check Scribe first because the prompt includes history which mentions other roles
	if strings.Contains(prompt, "You are the Scribe") {
		return "# Brainstorm Summary\n\n- PM wanted UI\n- Architect wanted microservices\n- QA wanted tests", nil
	}
	if strings.Contains(prompt, "You are Product Manager") || strings.Contains(prompt, "Product Manager") {
		return "PM: We need a user-friendly interface.", nil
	}
	if strings.Contains(prompt, "You are Architect") || strings.Contains(prompt, "Architect") {
		return "Architect: We should use microservices.", nil
	}
	if strings.Contains(prompt, "You are QA Engineer") || strings.Contains(prompt, "QA Engineer") {
		return "QA: We need comprehensive tests.", nil
	}

	return fmt.Sprintf("I don't know who I am. Prompt: %s", prompt), nil
}

func (m *MockAgentForBrainstorm) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Not used in brainstormCmd currently
	return m.Send(ctx, prompt)
}

func TestBrainstormCmd(t *testing.T) {
	// 1. Setup
	tmpDir, err := os.MkdirTemp("", "recac-brainstorm-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mockAgent := &MockAgentForBrainstorm{
		Prompts: []string{},
	}

	// Save original factory and restore it after test
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	// Override factory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 2. Prepare Session and Command
	outputFile := filepath.Join(tmpDir, "summary.md")

	session := &BrainstormSession{
		Topic:    "Test Topic",
		Personas: defaultPersonas,
	}

	// Create a dummy command just for context/output
	cmd := &cobra.Command{}
	buf := new(strings.Builder)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Register flags on this dummy command so runBrainstorm can read them
	cmd.Flags().Int("rounds", 1, "")

	// 3. Execute
	err = runBrainstorm(cmd, session, outputFile)
	require.NoError(t, err)

	// 4. Verify
	// Check file existence
	require.FileExists(t, outputFile)

	// Check file content
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "# Brainstorm Summary")
	assert.Contains(t, string(content), "PM wanted UI")

	// Check Prompts
	// We expect 1 round * 3 personas + 1 scribe = 4 calls
	assert.Equal(t, 4, len(mockAgent.Prompts), "Expected 4 prompts sent to agent")

	// Verify order/content roughly
	pmPromptFound := false
	scribePromptFound := false
	for _, p := range mockAgent.Prompts {
		if strings.Contains(p, "Product Manager") {
			pmPromptFound = true
		}
		if strings.Contains(p, "Scribe") {
			scribePromptFound = true
		}
	}
	assert.True(t, pmPromptFound, "Product Manager prompt not found")
	assert.True(t, scribePromptFound, "Scribe prompt not found")

	// Check stdout
	output := buf.String()
	assert.Contains(t, output, "Starting brainstorming session")
	assert.Contains(t, output, "Product Manager is thinking...")
	assert.Contains(t, output, "Brainstorming complete!")
}
