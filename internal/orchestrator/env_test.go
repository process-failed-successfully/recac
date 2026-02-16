package orchestrator

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildAgentEnvVars(t *testing.T) {
	// Setup environment
	os.Setenv("RECAC_NOTIFICATIONS_SLACK_ENABLED", "true")
	os.Setenv("JIRA_API_TOKEN", "secret-token")
	os.Setenv("RECAC_MAX_ITERATIONS", "30")
	defer func() {
		os.Unsetenv("RECAC_NOTIFICATIONS_SLACK_ENABLED")
		os.Unsetenv("JIRA_API_TOKEN")
		os.Unsetenv("RECAC_MAX_ITERATIONS")
	}()

	item := WorkItem{
		ID: "TEST-1",
		EnvVars: map[string]string{
			"CUSTOM_VAR": "custom-val",
		},
	}

	env := BuildAgentEnvVars(item, "openai", "gpt-4")

	assert.Equal(t, "openai", env["RECAC_PROVIDER"])
	assert.Equal(t, "gpt-4", env["RECAC_MODEL"])
	assert.Equal(t, "TEST-1", env["RECAC_PROJECT_ID"])
	assert.Equal(t, "custom-val", env["CUSTOM_VAR"])
	assert.Equal(t, "true", env["RECAC_NOTIFICATIONS_SLACK_ENABLED"])
	assert.Equal(t, "secret-token", env["JIRA_API_TOKEN"])
	assert.Equal(t, "30", env["RECAC_MAX_ITERATIONS"])
	assert.Equal(t, "0", env["GIT_TERMINAL_PROMPT"])
	assert.Equal(t, "RECAC Agent", env["GIT_AUTHOR_NAME"])
}
