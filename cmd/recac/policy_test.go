package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/agent"
)

type PolicyMockAgent struct {
	Response string
}

func (m *PolicyMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *PolicyMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestPolicyAddList(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-policy-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Chdir to temp dir to use local .recac folder
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Test Add
	// We call RunE directly to avoid Cobra parsing issues in test environment
	err = policyAddCmd.RunE(policyAddCmd, []string{"Test Policy"})
	assert.NoError(t, err)

	// Verify file
	policyFile := filepath.Join(tmpDir, ".recac", "policies.yaml")
	assert.FileExists(t, policyFile)
	content, _ := os.ReadFile(policyFile)
	assert.Contains(t, string(content), "Test Policy")

	// Test List
	err = policyListCmd.RunE(policyListCmd, []string{})
	assert.NoError(t, err)
}

func TestPolicyCheck_Fail(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-policy-check-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Chdir
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create a dummy file
	os.WriteFile("test.go", []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"fail\") }"), 0644)

	// Add policy
	policyAddCmd.RunE(policyAddCmd, []string{"No Println"})

	// Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &PolicyMockAgent{
			Response: `[{"file": "test.go", "line": 3, "policy": "No Println", "message": "Avoid fmt.Println"}]`,
		}, nil
	}

	// Run Check
	err = policyCheckCmd.RunE(policyCheckCmd, []string{})

	// Should fail because violations found
	assert.Error(t, err)
	assert.Equal(t, "policy check failed", err.Error())
}

func TestPolicyCheck_Pass(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-policy-pass-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Chdir
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Add policy
	policyAddCmd.RunE(policyAddCmd, []string{"No Println"})

	// Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &PolicyMockAgent{
			Response: `[]`,
		}, nil
	}

	// Run Check
	err = policyCheckCmd.RunE(policyCheckCmd, []string{})

	// Should pass
	assert.NoError(t, err)
}
