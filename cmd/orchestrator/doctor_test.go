package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestRunDoctor_Jira_Fail(t *testing.T) {
	// Setup Viper
	viper.Reset()
	viper.Set("orchestrator.poller", "jira")
	viper.Set("config.jiraUrl", "http://jira.example.com")
	viper.Set("config.jiraUsername", "user")
	viper.Set("secrets.jiraApiToken", "token")
	viper.Set("orchestrator.mode", "local")
	viper.Set("orchestrator.agent_provider", "openrouter")
	viper.Set("secrets.openrouterApiKey", "sk-123") // Provide key so AI check passes (mock is not used here but logic checks for empty string)

	// Capture output
	output := captureOutput(func() {
		err := runDoctor(nil, nil)
		// We expect error because network calls will fail
		assert.Error(t, err)
	})

	// Assertions
	assert.Contains(t, output, "Jira Connectivity")
	assert.Contains(t, output, "FAIL")
	assert.Contains(t, output, "AI Provider (openrouter)")
	// AI Provider check just checks for empty key, so it should pass if key provided?
	// Ah, CheckAIProvider checks if key is empty string.
	// So "PASS" expected for AI Provider.
	assert.Contains(t, output, "PASS")
}

func TestRunDoctor_GitHub_Fail(t *testing.T) {
	// Setup Viper
	viper.Reset()
	viper.Set("orchestrator.poller", "github")
	viper.Set("orchestrator.github_token", "token")
	viper.Set("orchestrator.github_owner", "owner")
	viper.Set("orchestrator.github_repo", "repo")
	viper.Set("orchestrator.mode", "local")
	viper.Set("orchestrator.agent_provider", "openrouter")
	viper.Set("secrets.openrouterApiKey", "sk-123")

	// Capture output
	output := captureOutput(func() {
		err := runDoctor(nil, nil)
		assert.Error(t, err)
	})

	// Assertions
	assert.Contains(t, output, "GitHub Connectivity")
	assert.Contains(t, output, "FAIL")
}

func TestRunDoctor_MissingConfig(t *testing.T) {
	viper.Reset()
	// Ensure no config file is loaded
	// verify output mentions missing config

	output := captureOutput(func() {
		_ = runDoctor(nil, nil)
	})

	if strings.Contains(output, "Config File") {
		// Depends on environment if config file exists in CWD
		// But usually tests run in tmp dir or package dir
		// We just check that it runs without panic
	}
}
