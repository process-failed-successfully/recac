package main

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestListKeysCommand(t *testing.T) {
	// Reset viper to ensure a clean state for the test
	viper.Reset()
	defer viper.Reset()

	// Set some sample configuration values
	viper.Set("agent.provider", "gemini")
	viper.Set("agent.model", "gemini-1.5-pro")
	viper.Set("api_key", "test-api-key") // This is a sensitive key

	// Execute the list-keys command
	output, err := executeCommand(rootCmd, "config", "list-keys")
	require.NoError(t, err)

	// Check that the output contains the expected keys and values using regex
	require.Regexp(t, `agent\.provider\s+gemini`, output)
	require.Regexp(t, `agent\.model\s+gemini-1.5-pro`, output)

	// Check that the sensitive key is redacted
	require.Regexp(t, `api_key\s+\[REDACTED\]`, output)
	require.NotContains(t, output, "test-api-key")
}

func TestListModelsCommand(t *testing.T) {
	// Execute the list-models command
	output, err := executeCommand(rootCmd, "config", "list-models")
	require.NoError(t, err)

	// Check that the output contains the expected providers and models using regex
	require.Contains(t, output, "Provider: Openai")
	require.Regexp(t, `GPT-4o\s+gpt-4o`, output)

	require.Contains(t, output, "Provider: Gemini")
	require.Regexp(t, `Gemini 2.0 Pro\s+gemini-2.0-pro`, output)

	require.Contains(t, output, "Provider: Ollama")
	require.Regexp(t, `Llama 3\s+llama3`, output)

	// Check for a few other providers to be safe
	require.Contains(t, output, "Provider: Anthropic")
	require.Contains(t, output, "Provider: Openrouter")
}
