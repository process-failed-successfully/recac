package main

import (
	"context"
	"errors"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

func TestGenerateTestsCmd_FileReadError(t *testing.T) {
	// Setup
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()

	readFileFunc = func(name string) ([]byte, error) {
		return nil, errors.New("read failed")
	}

	// Execute
	_, err := executeCommand(rootCmd, "generate-tests", "file.go")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestGenerateTestsCmd_EmptyFile(t *testing.T) {
	// Setup
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()

	readFileFunc = func(name string) ([]byte, error) {
		return []byte(""), nil
	}

	// Execute
	_, err := executeCommand(rootCmd, "generate-tests", "file.go")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is empty")
}

func TestGenerateTestsCmd_AgentInitError(t *testing.T) {
	// Setup
	// We need a file with content
	file := "file.go"
	// We don't mock readFileFunc here, we rely on os.ReadFile default via shared_utils (wait, we replaced it?)
	// If we replaced it in previous test, it's restored by defer.
	// But generate_tests.go uses readFileFunc.
	// We can write real file and let readFileFunc read it (if it points to os.ReadFile).
	// But to be safe and isolated, let's mock readFileFunc.

	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("package main"), nil
	}

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, errors.New("agent init failed")
	}

	// Execute
	_, err := executeCommand(rootCmd, "generate-tests", file)

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create agent")
}

func TestGenerateTestsCmd_AgentSendError(t *testing.T) {
	// Setup
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("package main"), nil
	}

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockErrorAgent{Err: errors.New("send failed")}, nil
	}

	// Execute
	_, err := executeCommand(rootCmd, "generate-tests", "file.go")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent failed")
}

func TestGenerateTestsCmd_WriteFileError(t *testing.T) {
	// Setup
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("package main"), nil
	}

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAg := agent.NewMockAgent()
	mockAg.SetResponse("```go\nfunc Test() {}\n```")
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	originalWriteFile := writeFileFunc
	defer func() { writeFileFunc = originalWriteFile }()
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		return errors.New("write failed")
	}

	// Execute
	_, err := executeCommand(rootCmd, "generate-tests", "file.go", "--output", "test.go")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write output file")
}

func TestGenerateTestsCmd_AutoFixNoOutput(t *testing.T) {
	// Setup
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()
	readFileFunc = func(name string) ([]byte, error) {
		return []byte("package main"), nil
	}

	// Execute
	_, err := executeCommand(rootCmd, "generate-tests", "file.go", "--auto-fix")

	// Verify
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "--auto-fix requires --output")
}
