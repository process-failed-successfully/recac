package main

import (
	"fmt"
	"os"
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

	// Teardown: Restore original values and clean up files
	defer func() {
		askOneFunc = originalAskOne
		viper.SetConfigFile(originalViperConfig)
		runDoctorFunc = originalRunDoctor
		os.Remove("test_config.yaml")
		// We remove .env only if we created it. Since the test creates it, we remove it.
		// If it existed before, we backed it up.
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
		"Do you want to save the API Key to a local .env file?": true,
		"Enable Slack notifications?":                           true,
		"Slack Channel:":                                        "#alerts",
		"Slack Bot Token:":                                      "xoxb-test",
		"Run system check (recac doctor) now?":                  true, // Changed to true to test doctor execution
	}

	// Mock the AskOne function
	askOneFunc = mockAskOne

	// Prepare environment
	viper.Reset()
	viper.SetConfigFile("test_config.yaml")

	// Backup .env if exists
	if _, err := os.Stat(".env"); err == nil {
		os.Rename(".env", ".env.bak")
		defer os.Rename(".env.bak", ".env")
	}

	// Execute command
	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	// Verify Viper settings (which would be written to config.yaml)
	assert.Equal(t, "openai", viper.GetString("provider"))
	assert.Equal(t, "gpt-4o", viper.GetString("model"))
	assert.True(t, viper.GetBool("notifications.slack.enabled"))
	assert.Equal(t, "#alerts", viper.GetString("notifications.slack.channel"))

	// Verify config file creation
	_, err = os.Stat("test_config.yaml")
	assert.NoError(t, err, "config file should exist")

	// Verify .env content
	envContent, err := os.ReadFile(".env")
	assert.NoError(t, err, ".env file should exist")
	content := string(envContent)
	assert.Contains(t, content, "OPENAI_API_KEY=sk-test-123")
	assert.Contains(t, content, "SLACK_BOT_USER_TOKEN=xoxb-test")

	// Cleanup .env created by test
	os.Remove(".env")
}
