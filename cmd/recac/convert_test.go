package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockConvertAgent struct {
	CapturedPrompt string
	Response       string
	Err            error
}

func (m *MockConvertAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CapturedPrompt = prompt
	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}

func (m *MockConvertAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestConvertCmd_Stdout(t *testing.T) {
	// Mock agent
	mockAgent := &MockConvertAgent{
		Response: "{\"name\": \"test\"}",
	}

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Mock file reading
	origReadFile := readFileFunc
	readFileFunc = func(filename string) ([]byte, error) {
		if filename == "test.yaml" {
			return []byte("name: test"), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFileFunc = origReadFile }()

	// Execute command
	output, err := executeCommand(rootCmd, "convert", "test.yaml", "json")
	require.NoError(t, err)

	// Assertions
	assert.Contains(t, output, "{\"name\": \"test\"}")
	assert.Contains(t, mockAgent.CapturedPrompt, "Convert the following content to json format")
	assert.Contains(t, mockAgent.CapturedPrompt, "name: test")
}

func TestConvertCmd_OutputFile(t *testing.T) {
	// Mock agent
	mockAgent := &MockConvertAgent{
		Response: "<converted_csv>",
	}

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Mock file reading
	origReadFile := readFileFunc
	readFileFunc = func(filename string) ([]byte, error) {
		if filename == "test.json" {
			return []byte("[]"), nil
		}
		return nil, os.ErrNotExist
	}
	defer func() { readFileFunc = origReadFile }()

	// Mock file writing
	var writtenContent string
	var writtenFile string
	origWriteFile := writeFileFunc
	writeFileFunc = func(name string, data []byte, perm fs.FileMode) error {
		writtenFile = name
		writtenContent = string(data)
		return nil
	}
	defer func() { writeFileFunc = origWriteFile }()

	// Execute command
	_, err := executeCommand(rootCmd, "convert", "test.json", "csv", "output.csv")
	require.NoError(t, err)

	// Assertions
	assert.Equal(t, "output.csv", writtenFile)
	assert.Equal(t, "<converted_csv>", writtenContent)
	assert.Contains(t, mockAgent.CapturedPrompt, "Convert the following content to csv format")
}

func TestConvertCmd_AgentFailure(t *testing.T) {
	// Mock agent
	mockAgent := &MockConvertAgent{
		Err: errors.New("agent failed"),
	}

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Mock file reading
	origReadFile := readFileFunc
	readFileFunc = func(filename string) ([]byte, error) {
		return []byte("dummy content"), nil
	}
	defer func() { readFileFunc = origReadFile }()

	// Execute command
	_, err := executeCommand(rootCmd, "convert", "in.txt", "out_format")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed to convert file: agent failed")
}

func TestConvertCmd_ReadFailure(t *testing.T) {
	// Mock file reading
	origReadFile := readFileFunc
	readFileFunc = func(filename string) ([]byte, error) {
		return nil, os.ErrNotExist
	}
	defer func() { readFileFunc = origReadFile }()

	// Execute command
	_, err := executeCommand(rootCmd, "convert", "nonexistent.yaml", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read input file")
}

func TestConvertCmd_EmptyInput(t *testing.T) {
	// Mock file reading
	origReadFile := readFileFunc
	readFileFunc = func(filename string) ([]byte, error) {
		return []byte(""), nil
	}
	defer func() { readFileFunc = origReadFile }()

	// Execute command
	_, err := executeCommand(rootCmd, "convert", "empty.yaml", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "input file is empty")
}
