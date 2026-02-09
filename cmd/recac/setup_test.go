package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// Mock askOneFunc
var mockAnswers map[string]interface{}

func mockAskOne(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	var q string
	switch v := p.(type) {
	case *survey.Select:
		q = v.Message
	case *survey.Input:
		q = v.Message
	case *survey.Password:
		q = v.Message
	case *survey.Confirm:
		q = v.Message
	default:
		return fmt.Errorf("unsupported prompt type")
	}

	ans, ok := mockAnswers[q]
	if !ok {
		return fmt.Errorf("unexpected prompt: %s", q)
	}

	// Helper to set response
	switch v := response.(type) {
	case *string:
		*v = ans.(string)
	case *bool:
		*v = ans.(bool)
	default:
		return fmt.Errorf("unsupported response type")
	}

	return nil
}

func TestSetupCmd(t *testing.T) {
	// Create temp home
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Backup original functions and env
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	// Mock Doctor
	originalRunDoctor := runDoctorFunc
	doctorRunCalled := false
	runDoctorFunc = func(cmd *cobra.Command, args []string) {
		doctorRunCalled = true
	}
	defer func() { runDoctorFunc = originalRunDoctor }()

	// Clean up .env file in current directory (since setup writes .env to CWD)
	os.Remove(".env")
	defer os.Remove(".env")

	viper.Reset()

	// Setup mock answers
	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                              "openai",
		"Enter the Model name:":                                 "gpt-4o",
		"Enter your API Key (leave empty to skip):":             "sk-test-123",
		"Do you want to save the API Key to a local .env file?": true,
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "https://example.atlassian.net",
		"Enter your Jira Email/Username:":                                "user@example.com",
		"Enter your Jira API Token:":                                     "jira-token-123",
		"Do you want to save the Jira Token to a local .env file?":       true,
		"Enter the Jira Label for agents to watch:":                      "recac-agent",
		"Enable Slack notifications?":                                    true,
		"Slack Channel:":                                                 "#alerts",
		"Slack Bot Token:":                                               "xoxb-test",
		"Run system check (recac doctor) now?":                           true,
	}

	// Mock the AskOne function
	askOneFunc = mockAskOne

	// Run command
	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	// Verify Doctor was called
	assert.True(t, doctorRunCalled, "Doctor command should have been executed")

	// Verify Config File Created in tmpHome
	expectedConfig := filepath.Join(tmpHome, ".recac.yaml")
	_, err = os.Stat(expectedConfig)
	assert.NoError(t, err, ".recac.yaml should be created in HOME")

	// Verify Viper settings (which would be written to config.yaml)
	assert.Equal(t, "openai", viper.GetString("provider"))
	assert.Equal(t, "gpt-4o", viper.GetString("model"))
	assert.Equal(t, "https://example.atlassian.net", viper.GetString("jira.url"))
	assert.Equal(t, "user@example.com", viper.GetString("jira.username"))
	assert.Equal(t, "recac-agent", viper.GetString("orchestrator.jira_label"))
	assert.True(t, viper.GetBool("notifications.slack.enabled"))
	assert.Equal(t, "#alerts", viper.GetString("notifications.slack.channel"))

	// Verify .env created in CWD
	envContent, err := os.ReadFile(".env")
	assert.NoError(t, err, ".env file should exist")
	content := string(envContent)
	assert.Contains(t, content, "OPENAI_API_KEY=sk-test-123")
	assert.Contains(t, content, "JIRA_API_TOKEN=jira-token-123")
	assert.Contains(t, content, "SLACK_BOT_USER_TOKEN=xoxb-test")
}

func TestSetupCmd_Skips(t *testing.T) {
	// Create temp home
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	os.Remove(".env")
	defer os.Remove(".env")

	viper.Reset()

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                  "openai",
		"Enter the Model name:":                     "gpt-3.5",
		"Enter your API Key (leave empty to skip):": "", // skip
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "", // skip
		"Enable Slack notifications?":          false, // skip
		"Run system check (recac doctor) now?": false, // skip
	}
	askOneFunc = mockAskOne

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	assert.Equal(t, "openai", viper.GetString("provider"))
	assert.Empty(t, viper.GetString("jira.url"))
	assert.False(t, viper.GetBool("notifications.slack.enabled"))

	// Verify Config File Created in tmpHome even if skips
	expectedConfig := filepath.Join(tmpHome, ".recac.yaml")
	_, err = os.Stat(expectedConfig)
	assert.NoError(t, err, ".recac.yaml should be created in HOME")
}

func TestSetupCmd_JiraTokenInConfig(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                  "gemini",
		"Enter the Model name:":                     "gemini-pro",
		"Enter your API Key (leave empty to skip):": "",
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "https://jira.example.com",
		"Enter your Jira Email/Username:":                                "dev@example.com",
		"Enter your Jira API Token:":                                     "secret-token",
		"Do you want to save the Jira Token to a local .env file?":       false, // Save to config instead
		"Enter the Jira Label for agents to watch:":                      "recac-dev",
		"Enable Slack notifications?":                                    false,
		"Run system check (recac doctor) now?":                           false,
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile("test_config_jira_cfg.yaml")
	defer os.Remove("test_config_jira_cfg.yaml")

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	assert.Equal(t, "https://jira.example.com", viper.GetString("jira.url"))
	assert.Equal(t, "dev@example.com", viper.GetString("jira.username"))
	assert.Equal(t, "recac-dev", viper.GetString("orchestrator.jira_label"))
	assert.Equal(t, "secret-token", viper.GetString("jira.api_token")) // Should be in config
}

func TestSetupCmd_AppendEnv(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	// Create existing .env
	os.WriteFile(".env", []byte("EXISTING_VAR=foo\n"), 0600)
	defer os.Remove(".env")

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                  "openai",
		"Enter the Model name:":                     "gpt-4",
		"Enter your API Key (leave empty to skip):": "new-key",
		"Do you want to save the API Key to a local .env file?": true,
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "",
		"Enable Slack notifications?":          false,
		"Run system check (recac doctor) now?": false,
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile("test_config_append.yaml")
	defer os.Remove("test_config_append.yaml")

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(".env")
	str := string(content)
	assert.Contains(t, str, "EXISTING_VAR=foo")
	assert.Contains(t, str, "OPENAI_API_KEY=new-key")
}
