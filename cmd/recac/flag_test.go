package main

import (
	"context"
	"go/format"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlagAdd(t *testing.T) {
	// Setup temp config
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	f, err := os.Create(configFile)
	require.NoError(t, err)
	f.Close()

	viper.Reset()
	// No need to set viper here, we pass it via --config

	// Execute
	_, err = executeCommand(rootCmd, "flag", "add", "new-feature", "--enable", "--desc", "My new feature", "--config", configFile)
	require.NoError(t, err)

	// Verify - we must read from the file to verify persistence
	viper.Reset()
	viper.SetConfigFile(configFile)
	require.NoError(t, viper.ReadInConfig())

	require.True(t, viper.IsSet("feature_flags.new-feature"))
	flags := viper.GetStringMap("feature_flags")
	val := flags["new-feature"].(map[string]interface{})
	assert.Equal(t, true, val["enabled"])
	assert.Equal(t, "My new feature", val["description"])
}

func TestFlagList(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	viper.Reset()
	viper.SetConfigFile(configFile)
	viper.Set("feature_flags.alpha", map[string]interface{}{"enabled": true, "description": "Alpha feature"})
	viper.Set("feature_flags.beta", map[string]interface{}{"enabled": false, "description": "Beta feature"})
	require.NoError(t, viper.WriteConfig())

	output, err := executeCommand(rootCmd, "flag", "list", "--config", configFile)
	require.NoError(t, err)

	assert.Contains(t, output, "alpha")
	assert.Contains(t, output, "enabled")
	assert.Contains(t, output, "Alpha feature")
	assert.Contains(t, output, "beta")
	assert.Contains(t, output, "disabled")
}

func TestFlagCleanup(t *testing.T) {
	// Setup temp dir and switch to it
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)
	require.NoError(t, os.Chdir(tmpDir))

	// Setup config
	configFile := filepath.Join(tmpDir, "config.yaml")
	viper.Reset()
	viper.SetConfigFile(configFile)
	viper.Set("feature_flags.legacy-flag", map[string]interface{}{"enabled": true})
	require.NoError(t, viper.WriteConfig())

	// Setup file with usage
	sourceFile := filepath.Join(tmpDir, "main.go")
	originalCode := `package main
func main() {
	if config.IsSet("legacy-flag") {
		doOldThing()
	} else {
		doNewThing()
	}
}`
	require.NoError(t, os.WriteFile(sourceFile, []byte(originalCode), 0644))

	// Mock Agent
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	refactoredCode := `package main
func main() {
	doNewThing() // Refactored by AI
}`

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mock := agent.NewMockAgent()
		mock.SetResponse(refactoredCode)
		return mock, nil
	}

	// Execute
	output, err := executeCommand(rootCmd, "flag", "cleanup", "legacy-flag", "--config", configFile)
	require.NoError(t, err, "Output: %s", output)

	// Verify File Content
	content, err := os.ReadFile(sourceFile)
	require.NoError(t, err)

	expectedBytes, err := format.Source([]byte(refactoredCode))
	require.NoError(t, err)
	assert.Equal(t, string(expectedBytes), string(content))

	// Verify Config Update
	// Note: removeFlagFromConfig reloads from viper state, deletes, and saves.
	// Since we are using the same viper instance, it should be reflected.
	flags := viper.GetStringMap("feature_flags")
	_, exists := flags["legacy-flag"]
	assert.False(t, exists, "flag should be removed from viper state")
}
