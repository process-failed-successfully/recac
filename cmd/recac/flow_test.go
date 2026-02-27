package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-flow-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a sample Go file
	srcFile := filepath.Join(tmpDir, "logic.go")
	srcContent := `package main

func processData(data int) string {
	if data > 10 {
		return "large"
	}
	return "small"
}
`
	err = os.WriteFile(srcFile, []byte(srcContent), 0644)
	require.NoError(t, err)

	// Switch working dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Mock Agent
	mockResponse := "graph TD\n  A[Start] --> B{data > 10}"
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse(mockResponse)

	// Override Factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Use Factory
	cmd := NewFlowCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// We invoke with the function name
	cmd.SetArgs([]string{"processData", "--dir", tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "data > 10")
}

func TestFlowCmd_NotFound(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-flow-test-fail")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy file so walk has something to check, but NOT the function we want
	os.WriteFile(filepath.Join(tmpDir, "other.go"), []byte("package main\nfunc other() {}"), 0644)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Override Factory
	mockAgent := agent.NewMockAgent()
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Use Factory - Ensures clean state
	cmd := NewFlowCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"nonExistentFunc", "--dir", tmpDir})

	err = cmd.Execute()

	if err == nil {
		t.Logf("Unexpected success. Output: %s", buf.String())
	}

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find function")
}
