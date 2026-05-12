package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestLoad_SpecificFile(t *testing.T) {
	defer viper.Reset()

	// Write a temp file
	content := []byte(`provider: testprovider`)
	err := os.WriteFile("test_config.yaml", content, 0644)
	assert.NoError(t, err)
	defer os.Remove("test_config.yaml")

	Load("test_config.yaml")
	assert.Equal(t, "testprovider", viper.GetString("provider"))
}

func TestGetDefaultConfig_JiraURL(t *testing.T) {
	t.Setenv("RECAC_JIRA_URL", "")
	t.Setenv("JIRA_URL", "http://myjira.com")

	defaults := GetDefaultConfig()
	assert.Equal(t, "http://myjira.com", defaults["jira.url"])
}
