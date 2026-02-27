package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Use LocalMockAgent to avoid conflict with MockAgent in tickets_test.go
type LocalMockAgent struct {
	mock.Mock
}

func (m *LocalMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *LocalMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func (m *LocalMockAgent) GetState() interface{} {
	return nil
}

func TestTranslateCmd_Success(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock readFileFunc
	origReadFile := readFileFunc
	readFileFunc = func(name string) ([]byte, error) {
		assert.Equal(t, "source.py", name)
		return []byte("print('hello')"), nil
	}
	defer func() { readFileFunc = origReadFile }()

	// Mock Agent
	mockAgent := new(LocalMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("fmt.Println(\"hello\")", nil)

	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// Run
	output, err := executeCommand(cmd, "translate", "source.py", "--target", "go")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "fmt.Println(\"hello\")")
	mockAgent.AssertExpectations(t)
}

func TestTranslateCmd_WithOutput(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock readFileFunc
	origReadFile := readFileFunc
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("print('hello')"), nil
	}
	defer func() { readFileFunc = origReadFile }()

	// Mock writeFileFunc
	origWriteFile := writeFileFunc
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		assert.Equal(t, "dest.go", name)
		// CleanCodeBlock returns string without newline, and writeFile writes exact bytes
		assert.Equal(t, "fmt.Println(\"hello\")", string(data))
		return nil
	}
	defer func() { writeFileFunc = origWriteFile }()

	// Mock Agent
	mockAgent := new(LocalMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("fmt.Println(\"hello\")", nil)

	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// Run
	output, err := executeCommand(cmd, "translate", "source.py", "--target", "go", "--output", "dest.go")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "Translated code saved to dest.go")
}

func TestTranslateCmd_FileNotFound(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock readFileFunc
	origReadFile := readFileFunc
	readFileFunc = func(name string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	defer func() { readFileFunc = origReadFile }()

	// Run
	_, err := executeCommand(cmd, "translate", "missing.py", "--target", "go")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestTranslateCmd_AgentFailure(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock readFileFunc
	origReadFile := readFileFunc
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("code"), nil
	}
	defer func() { readFileFunc = origReadFile }()

	// Mock Agent
	mockAgent := new(LocalMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("", errors.New("agent overloaded"))

	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// Run
	_, err := executeCommand(cmd, "translate", "source.py", "--target", "go")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
}

func TestTranslateCmd_MissingTarget(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Run without target flag (it's required)
	_, err := executeCommand(cmd, "translate", "source.py")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag(s) \"target\" not set")
}
