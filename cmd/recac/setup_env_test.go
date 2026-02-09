package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestSetupCmd_EnvVarSubstringMatch(t *testing.T) {
	originalAskOne := askOneFunc
	defer func() { askOneFunc = originalAskOne }()

	// Create existing .env with a variable that ends with the key we want to add
	// e.g. MY_OPENAI_API_KEY should not prevent OPENAI_API_KEY from being added.
	os.WriteFile(".env", []byte("MY_OPENAI_API_KEY=existing\n"), 0600)
	defer os.Remove(".env")

	mockAnswers = map[string]interface{}{
		"Choose your AI Provider:":                              "openai",
		"Enter the Model name:":                                 "gpt-4",
		"Enter your API Key (leave empty to skip):":             "new-key",
		"Do you want to save the API Key to a local .env file?": true,
		"Enter your Jira URL (e.g., https://your-domain.atlassian.net):": "",
		"Enable Slack notifications?":          false,
		"Run system check (recac doctor) now?": false,
	}
	askOneFunc = mockAskOne

	viper.Reset()
	viper.SetConfigFile("test_config_repro.yaml")
	defer os.Remove("test_config_repro.yaml")

	cmd := &cobra.Command{Use: "test"}
	err := runSetup(cmd, []string{})
	assert.NoError(t, err)

	content, _ := os.ReadFile(".env")
	str := string(content)
	// We expect OPENAI_API_KEY to be added because MY_OPENAI_API_KEY is different.
	assert.Contains(t, str, "OPENAI_API_KEY=new-key")
}
