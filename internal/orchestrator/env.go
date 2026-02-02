package orchestrator

import (
	"os"
)

// BuildAgentEnvVars constructs a map of environment variables to be injected into the agent.
// It includes:
// - Provider and Model configuration
// - WorkItem specific variables
// - Common secrets and API keys
// - Git identity configuration
// - Agent operational limits
// - Notification settings
func BuildAgentEnvVars(item WorkItem, provider, model string) map[string]string {
	env := make(map[string]string)

	// 1. Provider and Model
	if provider != "" {
		env["RECAC_PROVIDER"] = provider
	}
	if model != "" {
		env["RECAC_MODEL"] = model
	}

	// 2. WorkItem specific variables
	for k, v := range item.EnvVars {
		env[k] = v
	}
	env["RECAC_PROJECT_ID"] = item.ID

	// 3. Standard Git Config
	env["GIT_TERMINAL_PROMPT"] = "0"

	// 4. Secrets and API Keys (Propagated from Host)
	secrets := []string{
		"JIRA_API_TOKEN", "JIRA_USERNAME", "JIRA_URL",
		"GITHUB_TOKEN", "GITHUB_API_KEY",
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY",
		"RECAC_DB_TYPE", "RECAC_DB_URL",
	}
	for _, secret := range secrets {
		if val := os.Getenv(secret); val != "" {
			env[secret] = val
			// Special handling for GITHUB_API_KEY duplication
			if secret == "GITHUB_API_KEY" {
				env["RECAC_GITHUB_API_KEY"] = val
			}
		}
	}

	// 5. Notification Settings
	notifications := []string{
		"RECAC_NOTIFICATIONS_DISCORD_ENABLED",
		"RECAC_NOTIFICATIONS_SLACK_ENABLED",
	}
	for _, note := range notifications {
		if val := os.Getenv(note); val != "" {
			env[note] = val
		}
	}

	// 6. Agent Limits
	// Default RECAC_MAX_ITERATIONS to 20 if not set
	if val := os.Getenv("RECAC_MAX_ITERATIONS"); val != "" {
		env["RECAC_MAX_ITERATIONS"] = val
	} else {
		env["RECAC_MAX_ITERATIONS"] = "20"
	}

	otherLimits := []string{
		"RECAC_MANAGER_FREQUENCY",
		"RECAC_TASK_MAX_ITERATIONS",
	}
	for _, limit := range otherLimits {
		if val := os.Getenv(limit); val != "" {
			env[limit] = val
		}
	}

	// 7. Git Identity
	env["GIT_AUTHOR_NAME"] = "RECAC Agent"
	env["GIT_AUTHOR_EMAIL"] = "agent@recac.io"
	env["GIT_COMMITTER_NAME"] = "RECAC Agent"
	env["GIT_COMMITTER_EMAIL"] = "agent@recac.io"

	return env
}
