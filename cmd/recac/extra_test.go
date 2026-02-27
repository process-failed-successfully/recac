package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunSetup(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Set flags
	setupProvider = "openai"
	setupAPIKey = "sk-test"
	setupModel = "gpt-4"
	setupJiraURL = "https://jira.example.com"
	setupJiraToken = "jira-token"

	// Reset viper
	viper.Reset()
	configFile := filepath.Join(tmpDir, "config.yaml")
	viper.SetConfigFile(configFile)

	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))

	err := runSetup(cmd, []string{})
	require.NoError(t, err)

	// Verify config file
	assert.FileExists(t, configFile)

	// Verify .env file
	assert.FileExists(t, ".env")
	envContent, _ := os.ReadFile(".env")
	assert.Contains(t, string(envContent), "OPENAI_API_KEY=sk-test")
	assert.Contains(t, string(envContent), "JIRA_API_TOKEN=jira-token")
}

func TestRunRoast(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "bad_code.go"), []byte("package main\nfunc main() { var x = 1; }"), 0644)

	// Mock Agent
	mockAgent := &MockAgentCommit{}
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("This code smells!", nil)

	originalAgentFactory := roastAgentFactory
	roastAgentFactory = func(ctx context.Context, p, m, pp, pn string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { roastAgentFactory = originalAgentFactory }()

	// Execute
	cmd := &cobra.Command{Use: "roast", RunE: runRoast}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set args (target file or dir)
	// If roast takes args for files
	err := runRoast(cmd, []string{filepath.Join(tmpDir, "bad_code.go")})
	require.NoError(t, err)

	assert.Contains(t, outBuf.String(), "This code smells!")
}
