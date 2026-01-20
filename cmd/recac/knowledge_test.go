package main

import (
	"bytes"
	"context"
	"os"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
)

// KnowledgeTestMockAgent implements agent.Agent for testing
type KnowledgeTestMockAgent struct {
	Response string
}

func (m *KnowledgeTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *KnowledgeTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestKnowledgeCommands(t *testing.T) {
	// Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-knowledge-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change WD
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)

	// Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &KnowledgeTestMockAgent{}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Test Add
	t.Run("Add Rule", func(t *testing.T) {
		cmd := knowledgeAddCmd
		// Reset flags if any (knowledgeAddCmd has none but args)
		err := cmd.RunE(cmd, []string{"Do not use global variables"})
		assert.NoError(t, err)

		content, err := os.ReadFile("AGENTS.md")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- Do not use global variables")
	})

	// Test List
	t.Run("List Rules", func(t *testing.T) {
		buf := new(bytes.Buffer)
		knowledgeListCmd.SetOut(buf)
		err := knowledgeListCmd.RunE(knowledgeListCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "- Do not use global variables")
	})

	// Test Check
	t.Run("Check Rules", func(t *testing.T) {
		// Create a file to check
		err := os.WriteFile("bad.go", []byte("var Global = 1"), 0644)
		assert.NoError(t, err)

		// Mock Agent to say PASS
		mockAgent.Response = "PASS"

		buf := new(bytes.Buffer)
		knowledgeCheckCmd.SetOut(buf)
		err = knowledgeCheckCmd.RunE(knowledgeCheckCmd, []string{"bad.go"})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "PASS")

		// Mock Agent to say FAIL
		mockAgent.Response = "Violation: Global variable found."
		buf.Reset()
		err = knowledgeCheckCmd.RunE(knowledgeCheckCmd, []string{"bad.go"})
		assert.Error(t, err)
		assert.Contains(t, buf.String(), "VIOLATIONS FOUND")
	})

	// Test Learn
	t.Run("Learn Rules", func(t *testing.T) {
		// Create some dummy code to "learn" from
		err := os.Mkdir("src", 0755)
		assert.NoError(t, err)
		err = os.WriteFile("src/main.go", []byte("package main\nfunc main() {}"), 0644)
		assert.NoError(t, err)

		// Mock Agent response
		mockAgent.Response = "- Use contexts in all handlers"

		// Mock Input for interactive confirmation (send "y")
		r, w, _ := os.Pipe()
		w.Write([]byte("y\n"))
		w.Close()

		// Save original stdin
		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()
		os.Stdin = r

		buf := new(bytes.Buffer)
		knowledgeLearnCmd.SetOut(buf)

		// Set flags directly on the command
		// Note: Viper/Cobra binding usually happens in Execute, but we are calling RunE directly.
		// `knowledgeLearnCmd` uses `knowledgeFocus` variable which is bound in `init()`.
		// Since we are running in the same process, we can just set the variable `knowledgeFocus` if it's exported or directly accessible.
		// It is package-level `var knowledgeFocus string`.
		knowledgeFocus = "src"

		err = knowledgeLearnCmd.RunE(knowledgeLearnCmd, []string{})
		assert.NoError(t, err)

		content, err := os.ReadFile("AGENTS.md")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- Use contexts in all handlers")
	})
}
