package orchestrator

import (
	"os"
)

// BuildAgentEnvVars constructs the common environment variables for the agent.
// It includes provider configuration, git settings, secrets, and notifications.
func BuildAgentEnvVars(item WorkItem, provider, model string) map[string]string {
	env := make(map[string]string)

	// 1. Copy item specific EnvVars
	for k, v := range item.EnvVars {
		env[k] = v
	}

	// 2. Provider Config
	if provider != "" {
		env["RECAC_PROVIDER"] = provider
	}
	if model != "" {
		env["RECAC_MODEL"] = model
	}

	// 3. Project ID
	env["RECAC_PROJECT_ID"] = item.ID

	// 4. Git Configuration
	env["GIT_TERMINAL_PROMPT"] = "0"
	env["GIT_AUTHOR_NAME"] = "RECAC Agent"
	env["GIT_AUTHOR_EMAIL"] = "agent@recac.io"
	env["GIT_COMMITTER_NAME"] = "RECAC Agent"
	env["GIT_COMMITTER_EMAIL"] = "agent@recac.io"

	// 5. Notifications Config
	// Propagate if set in host environment
	notificationVars := []string{
		"RECAC_NOTIFICATIONS_DISCORD_ENABLED",
		"RECAC_NOTIFICATIONS_SLACK_ENABLED",
	}
	for _, v := range notificationVars {
		if val := os.Getenv(v); val != "" {
			env[v] = val
		}
	}

	// 6. Secrets and API Keys
	// We propagate these from the host environment to the agent
	secrets := []string{
		"JIRA_API_TOKEN", "JIRA_USERNAME", "JIRA_URL",
		"GITHUB_TOKEN", "GITHUB_API_KEY",
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY",
		"RECAC_DB_TYPE", "RECAC_DB_URL",
	}

	for _, secret := range secrets {
		if val := os.Getenv(secret); val != "" {
			env[secret] = val
			// Special handling for GITHUB_API_KEY alias
			if secret == "GITHUB_API_KEY" {
				env["RECAC_GITHUB_API_KEY"] = val
			}
		}
	}

	// 7. Agent Limits and Tuning
	// Default max iterations if not set
	maxIterations := "20"
	if val := os.Getenv("RECAC_MAX_ITERATIONS"); val != "" {
		maxIterations = val
	}
	env["RECAC_MAX_ITERATIONS"] = maxIterations

	tuningVars := []string{
		"RECAC_MANAGER_FREQUENCY",
		"RECAC_TASK_MAX_ITERATIONS",
	}
	for _, v := range tuningVars {
		if val := os.Getenv(v); val != "" {
			env[v] = val
		}
	}

	return env
}
