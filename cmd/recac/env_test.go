package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestEnvCheck(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	examplePath := filepath.Join(tempDir, "example.env")

	// Case 1: Clean (Sync)
	os.WriteFile(envPath, []byte("FOO=bar\nBAZ=qux\n"), 0644)
	os.WriteFile(examplePath, []byte("FOO=val\nBAZ=val\n"), 0644)

	// Set globals
	envFile = envPath
	envExample = examplePath
	envDetailed = false

	err := runEnvCheck(&cobra.Command{}, []string{})
	assert.NoError(t, err)

	// Case 2: Missing keys
	os.WriteFile(envPath, []byte("FOO=bar\n"), 0644) // Missing BAZ
	err = runEnvCheck(&cobra.Command{}, []string{})
	assert.Error(t, err)
}

func TestEnvSync(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	examplePath := filepath.Join(tempDir, "example.env")

	os.WriteFile(envPath, []byte("FOO=bar\nNEW_KEY=secret\nPASSWORD=123456\n"), 0644)
	os.WriteFile(examplePath, []byte("FOO=val\n"), 0644)

	envFile = envPath
	envExample = examplePath

	err := runEnvSync(&cobra.Command{}, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(examplePath)
	strContent := string(content)

	assert.Contains(t, strContent, "FOO=val")
	assert.Contains(t, strContent, "NEW_KEY=") // Should be empty
	assert.Contains(t, strContent, "PASSWORD=your_password") // Should be sanitized
}

func TestEnvGenerate(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")
	examplePath := filepath.Join(tempDir, "example.env")

	os.WriteFile(examplePath, []byte("DB_HOST=localhost\nDB_PASS=secret\n"), 0644)

	envFile = envPath
	envExample = examplePath
	envForce = false

	// Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("DB_HOST=127.0.0.1\nDB_PASS=INSERT_HERE")

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Run Generate
	err := runEnvGenerate(&cobra.Command{}, []string{})
	assert.NoError(t, err)

	// Check result
	content, _ := os.ReadFile(envPath)
	strContent := string(content)

	assert.Contains(t, strContent, "DB_HOST=127.0.0.1")
	assert.Contains(t, strContent, "DB_PASS=INSERT_HERE")

	// Check non-overwrite safety
	envForce = false
	err = runEnvGenerate(&cobra.Command{}, []string{})
	assert.Error(t, err) // Should fail because file exists

	// Check overwrite
	envForce = true
	err = runEnvGenerate(&cobra.Command{}, []string{})
	assert.NoError(t, err)
}

func TestMain(m *testing.M) {
	// Setup code usually not needed for simple unit tests but good practice to isolate if needed
	// Here we just run tests
	os.Exit(m.Run())
}
