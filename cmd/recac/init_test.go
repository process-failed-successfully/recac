package main

import (
	"bytes"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitCmd_Interactive(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-init-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change working directory to temp dir so config.yaml is written there
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Reset viper
	viper.Reset()

	// Prepare input
	// Inputs:
	// 1. Provider (enter for default gemini)
	// 2. Model (enter for default gemini-pro)
	// 3. API Key (input "my-secret-key")
	// 4. Jira URL (input "https://jira.example.com")
	// 5. Jira User (input "user@example.com")
	// 6. Jira Token (input "jira-token")
	inputData := "\n\nmy-secret-key\nhttps://jira.example.com\nuser@example.com\njira-token\n"
	input := bytes.NewBufferString(inputData)
	output := new(bytes.Buffer)

	// Run
	err = runInit(input, output)
	assert.NoError(t, err)

	// Verify Output contains prompts
	outStr := output.String()
	assert.Contains(t, outStr, "Welcome to RECAC Setup!")
	assert.Contains(t, outStr, "AI Provider")
	assert.Contains(t, outStr, "API Key")
	// Note: We might not see "Configuration saved to config.yaml" if runInit uses absolute path or similar,
	// but we expect it to try to save.

	// Verify Config File Created
	assert.FileExists(t, "config.yaml")

	// Verify Content
	viper.SetConfigFile("config.yaml")
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	assert.Equal(t, "gemini", viper.GetString("provider"))
	// runInit logic: if currentModel is empty, it picks default for provider.
	// For gemini, default is gemini-pro.
	assert.Equal(t, "gemini-pro", viper.GetString("model"))
	assert.Equal(t, "my-secret-key", viper.GetString("api_key"))
	assert.Equal(t, "https://jira.example.com", viper.GetString("jira.url"))
	assert.Equal(t, "user@example.com", viper.GetString("jira.username"))
	assert.Equal(t, "jira-token", viper.GetString("jira.api_token"))
}

func TestInitCmd_Overwrite(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-init-test-overwrite")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create existing config
	existingConfig := []byte(`
provider: openai
model: gpt-3.5
api_key: old-key
`)
	err = os.WriteFile("config.yaml", existingConfig, 0644)
	assert.NoError(t, err)

	// Reset viper and load existing
	viper.Reset()
	viper.SetConfigFile("config.yaml")
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	// Inputs:
	// 1. Provider (enter to keep openai)
	// 2. Model (enter to keep gpt-3.5)
	// 3. API Key (input "new-key")
	// 4. Jira URL (enter to skip)
	inputData := "\n\nnew-key\n\n"
	input := bytes.NewBufferString(inputData)
	output := new(bytes.Buffer)

	// Run
	err = runInit(input, output)
	assert.NoError(t, err)

	// Verify Content Updated
	// Force reload from disk
	viper.Reset()
	viper.SetConfigFile("config.yaml")
	err = viper.ReadInConfig()
	assert.NoError(t, err)

	assert.Equal(t, "openai", viper.GetString("provider"))
	assert.Equal(t, "gpt-3.5", viper.GetString("model"))
	assert.Equal(t, "new-key", viper.GetString("api_key"))
}
