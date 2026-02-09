package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// Mock input sequence for the test
var mockAnswers map[string]interface{}
var mockAnswersOrder []string
var mockAnswerIndex int

func mockAskOne(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
	// Determine which question is being asked to provide the correct mock answer
	var question string
	switch prompt := p.(type) {
	case *survey.Select:
		question = prompt.Message
	case *survey.Input:
		question = prompt.Message
	case *survey.Password:
		question = prompt.Message
	case *survey.Confirm:
		question = prompt.Message
	default:
		return fmt.Errorf("unknown prompt type")
	}

	// Find the mock answer based on the message
	val, ok := mockAnswers[question]
	if !ok {
		return fmt.Errorf("unexpected question: %s", question)
	}

	// Assign the value to the response pointer
	switch r := response.(type) {
	case *string:
		*r = val.(string)
	case *bool:
		*r = val.(bool)
	case *int:
		*r = val.(int)
	default:
		return fmt.Errorf("unsupported response type")
	}

	return nil
}

func TestSetupCmd(t *testing.T) {
	// Setup: Backup original values
	originalAskOne := askOneFunc
	originalViperConfig := viper.ConfigFileUsed()
	originalRunDoctor := runDoctorFunc
	originalEnvFile := setupEnvFile

	// Use temp dir for isolation
	tempDir := t.TempDir()
	setupEnvFile = filepath.Join(tempDir, ".env")
	testConfig := filepath.Join(tempDir, "test_config.yaml")

	// Teardown: Restore original values
	defer func() {
		askOneFunc = originalAskOne
		viper.SetConfigFile(originalViperConfig)
		runDoctorFunc = originalRunDoctor
		setupEnvFile = originalEnvFile
	}()

	// Mock Doctor execution
	runDoctorFunc = func(cmd *cobra.Command, args []string) {
		fmt.Println("Mock Doctor Executed")
	}

	// Define mock answers
	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                              "openai",
		"Enter the Model name:":                                 "gpt-4o",
		"Enter your API Key (leave empty to skip):":             "sk-test-123",
		fmt.Sprintf("Do you want to save the API Key to a local %s file?", setupEnvFile): true,
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):":                 "https://example.atlassian.net",
		"Enter your Jira Email/Username:":                                                "user@example.com",
		"Enter your Jira API Token:":                                                     "jira-token-123",
		fmt.Sprintf("Do you want to save the Jira Token to a local %s file?", setupEnvFile): true,
		"Enter the Jira Label for agents to watch:":                                         "recac-agent",
		"Enable Slack notifications?":                                                       true,
		"Slack Channel:":                                                                    "#alerts",
		"Slack Bot Token:":                                                                  "xoxb-test",
		"Run system check (recac doctor) now?":                                              true,
	}

	// Mock the AskOne function
	askOneFunc = mockAskOne

	// Prepare environment
	viper.Reset()
	viper.SetConfigFile(testConfig)

	// Execute command
	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	// Verify Viper settings (which would be written to config.yaml)
	assert.Equal(t, "openai", viper.GetString("provider"))
	assert.Equal(t, "gpt-4o", viper.GetString("model"))
	assert.Equal(t, "https://example.atlassian.net", viper.GetString("jira.url"))
	assert.Equal(t, "user@example.com", viper.GetString("jira.username"))
	assert.Equal(t, "recac-agent", viper.GetString("orchestrator.jira_label"))
	assert.True(t, viper.GetBool("notifications.slack.enabled"))
	assert.Equal(t, "#alerts", viper.GetString("notifications.slack.channel"))

	// Verify config file creation
	_, err = os.Stat(testConfig)
	assert.NoError(t, err, "config file should exist")

	// Verify .env content
	envContent, err := os.ReadFile(setupEnvFile)
	assert.NoError(t, err, ".env file should exist")
	content := string(envContent)
	assert.Contains(t, content, "OPENAI_API_KEY=sk-test-123")
	assert.Contains(t, content, "JIRA_API_TOKEN=jira-token-123")
	assert.Contains(t, content, "SLACK_BOT_USER_TOKEN=xoxb-test")
}

func TestSetupCmd_Cancellation(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		return errors.New("cancelled")
	}

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestSetupCmd_Skips(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	// Even for skips, we should isolate just in case
	originalEnvFile := setupEnvFile
	tempDir := t.TempDir()
	setupEnvFile = filepath.Join(tempDir, ".env")
	testConfig := filepath.Join(tempDir, "test_config_skips.yaml")

	defer func() { setupEnvFile = originalEnvFile }()

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                  "openai",
		"Enter the Model name:":                     "gpt-3.5",
		"Enter your API Key (leave empty to skip):": "", // skip
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "", // skip
		"Enable Slack notifications?":          false, // skip
		"Run system check (recac doctor) now?": false, // skip
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile(testConfig)

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	assert.Equal(t, "openai", viper.GetString("provider"))
	assert.Empty(t, viper.GetString("jira.url"))
	assert.False(t, viper.GetBool("notifications.slack.enabled"))
}

func TestSetupCmd_JiraTokenInConfig(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	originalEnvFile := setupEnvFile
	tempDir := t.TempDir()
	setupEnvFile = filepath.Join(tempDir, ".env")
	testConfig := filepath.Join(tempDir, "test_config_jira_cfg.yaml")

	defer func() { setupEnvFile = originalEnvFile }()

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                  "gemini",
		"Enter the Model name:":                     "gemini-pro",
		"Enter your API Key (leave empty to skip):": "",
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "https://jira.example.com",
		"Enter your Jira Email/Username:":                                "dev@example.com",
		"Enter your Jira API Token:":                                     "secret-token",
		fmt.Sprintf("Do you want to save the Jira Token to a local %s file?", setupEnvFile): false, // Save to config instead
		"Enter the Jira Label for agents to watch:":                                         "recac-dev",
		"Enable Slack notifications?":                                                       false,
		"Run system check (recac doctor) now?":                                              false,
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile(testConfig)

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

	originalEnvFile := setupEnvFile
	tempDir := t.TempDir()
	setupEnvFile = filepath.Join(tempDir, ".env")
	testConfig := filepath.Join(tempDir, "test_config_append.yaml")

	defer func() { setupEnvFile = originalEnvFile }()

	// Create existing .env
	os.WriteFile(setupEnvFile, []byte("EXISTING_VAR=foo\n"), 0600)

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                              "openai",
		"Enter the Model name:":                                 "gpt-4",
		"Enter your API Key (leave empty to skip):":             "new-key",
		fmt.Sprintf("Do you want to save the API Key to a local %s file?", setupEnvFile): true,
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):":                 "",
		"Enable Slack notifications?":                                                    false,
		"Run system check (recac doctor) now?":                                           false,
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile(testConfig)

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(setupEnvFile)
	str := string(content)
	assert.Contains(t, str, "EXISTING_VAR=foo")
	assert.Contains(t, str, "OPENAI_API_KEY=new-key")
}

func TestSetupCmd_PrefixCollision(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	originalEnvFile := setupEnvFile
	tempDir := t.TempDir()
	setupEnvFile = filepath.Join(tempDir, ".env")
	testConfig := filepath.Join(tempDir, "test_config_collision.yaml")

	defer func() { setupEnvFile = originalEnvFile }()

	// Create existing .env with a prefix collision
	os.WriteFile(setupEnvFile, []byte("MY_OPENAI_API_KEY=foo\n"), 0600)

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                              "openai",
		"Enter the Model name:":                                 "gpt-4",
		"Enter your API Key (leave empty to skip):":             "new-key",
		fmt.Sprintf("Do you want to save the API Key to a local %s file?", setupEnvFile): true,
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):":                 "",
		"Enable Slack notifications?":                                                    false,
		"Run system check (recac doctor) now?":                                           false,
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile(testConfig)

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(setupEnvFile)
	str := string(content)
	// It should contain the new key
	assert.Contains(t, str, "OPENAI_API_KEY=new-key")
}
