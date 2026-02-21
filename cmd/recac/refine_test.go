package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRefineAgent implements agent.Agent interface
type MockRefineAgent struct {
	mock.Mock
}

func (m *MockRefineAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockRefineAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestRefineCmd_InteractiveLoop(t *testing.T) {
	// 1. Setup Mock Agent
	mockAgent := new(MockRefineAgent)

	// Turn 1: Agent asks a question
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return true
	})).Return("QUESTION: What is the database?", nil).Once()

	// Turn 2: Agent finishes
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return true
	})).Return("DONE\nSPEC:\nUse Postgres DB", nil).Once()

	// 2. Mock Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 3. Setup Input/Output
	userInput := "Postgres\n"
	inBuf := bytes.NewBufferString(userInput)
	outBuf := new(bytes.Buffer)

	// 4. Setup Root Command
	// We use rootCmd to ensure arg parsing works as expected in real execution
	resetFlags(rootCmd)
	defer resetFlags(rootCmd)
	rootCmd.SetIn(inBuf)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(outBuf)

	rootCmd.SetArgs([]string{"refine", "I want a blog", "--output", "test_spec_loop.txt"})

	// 5. Execute
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// 6. Verify Output
	output := outBuf.String()
	assert.Contains(t, output, "What is the database?")
	assert.Contains(t, output, "Specification generated")

	// Verify Spec File
	content, err := os.ReadFile("test_spec_loop.txt")
	assert.NoError(t, err)
	assert.Equal(t, "Use Postgres DB", string(content))

	// Cleanup
	os.Remove("test_spec_loop.txt")
}

func TestRefineCmd_DoneImmediately(t *testing.T) {
	// 1. Setup Mock Agent
	mockAgent := new(MockRefineAgent)

	// Turn 1: Agent is satisfied immediately
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("DONE\nSPEC:\nSimple Spec", nil).Once()

	// 2. Mock Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 3. Setup Input/Output
	inBuf := bytes.NewBufferString("") // No interaction needed
	outBuf := new(bytes.Buffer)

	resetFlags(rootCmd)
	defer resetFlags(rootCmd)
	rootCmd.SetIn(inBuf)
	rootCmd.SetOut(outBuf)
	rootCmd.SetErr(outBuf)

	rootCmd.SetArgs([]string{"refine", "Just do it", "--output", "test_spec_immediate.txt"})

	// 5. Execute
	err := rootCmd.Execute()
	assert.NoError(t, err)

	// 6. Verify Output
	content, err := os.ReadFile("test_spec_immediate.txt")
	assert.NoError(t, err)
	assert.Equal(t, "Simple Spec", string(content))

	os.Remove("test_spec_immediate.txt")
}
