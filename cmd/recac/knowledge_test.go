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

	// Test List Rules Empty
	t.Run("List Rules Empty", func(t *testing.T) {
		buf := new(bytes.Buffer)
		knowledgeListCmd.SetOut(buf)
		err := knowledgeListCmd.RunE(knowledgeListCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "No AGENTS.md found")
	})

	// Test Add Rule
	t.Run("Add Rule", func(t *testing.T) {
		cmd := knowledgeAddCmd
		// Reset flags if any (knowledgeAddCmd has none but args)
		err := cmd.RunE(cmd, []string{"Do not use global variables"})
		assert.NoError(t, err)

		content, err := os.ReadFile("AGENTS.md")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- Do not use global variables")
	})

	// Test Add Second Rule (Cover append newline)
	t.Run("Add Second Rule", func(t *testing.T) {
		cmd := knowledgeAddCmd
		err := cmd.RunE(cmd, []string{"Keep functions small"})
		assert.NoError(t, err)

		content, err := os.ReadFile("AGENTS.md")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "\n- Keep functions small")
	})

	// Test List Rules
	t.Run("List Rules", func(t *testing.T) {
		buf := new(bytes.Buffer)
		knowledgeListCmd.SetOut(buf)
		err := knowledgeListCmd.RunE(knowledgeListCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "- Do not use global variables")
		assert.Contains(t, buf.String(), "- Keep functions small")
	})

	// Test Check Rules (File)
	t.Run("Check Rules File", func(t *testing.T) {
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

	// Test Check Rules (Stdin)
	t.Run("Check Rules Stdin", func(t *testing.T) {
		// Mock Agent to say PASS
		mockAgent.Response = "PASS"

		// Mock Stdin
		r, w, _ := os.Pipe()
		w.Write([]byte("package main\n"))
		w.Close()

		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()
		os.Stdin = r

		buf := new(bytes.Buffer)
		knowledgeCheckCmd.SetOut(buf)
		err := knowledgeCheckCmd.RunE(knowledgeCheckCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "PASS")
	})

	// Test Learn Rules
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
		knowledgeFocus = "src"

		err = knowledgeLearnCmd.RunE(knowledgeLearnCmd, []string{})
		assert.NoError(t, err)

		content, err := os.ReadFile("AGENTS.md")
		assert.NoError(t, err)
		assert.Contains(t, string(content), "- Use contexts in all handlers")
	})

	// Test Learn Rules Abort
	t.Run("Learn Rules Abort", func(t *testing.T) {
		mockAgent.Response = "- Aborted Rule"

		// Mock Input for interactive confirmation (send "n")
		r, w, _ := os.Pipe()
		w.Write([]byte("n\n"))
		w.Close()

		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }()
		os.Stdin = r

		buf := new(bytes.Buffer)
		knowledgeLearnCmd.SetOut(buf)
		knowledgeFocus = "src"

		err := knowledgeLearnCmd.RunE(knowledgeLearnCmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "Aborted")

		content, err := os.ReadFile("AGENTS.md")
		assert.NoError(t, err)
		assert.NotContains(t, string(content), "- Aborted Rule")
	})
}