package main

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestConfigSecretsCommand(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("api_key", "secret123")
	viper.Set("agent.provider", "gemini")
	viper.Set("orchestrator.github_token", "github123")
	viper.Set("some_secret", "")

	output, err := executeCommand(rootCmd, "config", "secrets")
	require.NoError(t, err)

	require.Contains(t, output, "SENSITIVE KEY")
	require.Contains(t, output, "STATUS")

	// Verify sensitive keys are listed
	require.Contains(t, output, "api_key")
	require.Contains(t, output, "orchestrator.github_token")
	require.Contains(t, output, "some_secret")

	// Verify non-sensitive keys are NOT listed
	require.NotContains(t, output, "agent.provider")

	// Verify status
	require.Regexp(t, `api_key\s+Set`, output)
	require.Regexp(t, `orchestrator\.github_token\s+Set`, output)
	require.Regexp(t, `some_secret\s+Not Set`, output)
}
