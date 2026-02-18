package orchestrator

import (
	"os"
)

// collectAgentEnvVars gathers common environment variables for the agent.
// Precedence: Host Config > Defaults > WorkItem EnvVars
func collectAgentEnvVars(item WorkItem, provider, model string) map[string]string {
	env := make(map[string]string)

	// 1. User provided env vars (Base)
	for k, v := range item.EnvVars {
		env[k] = v
	}

	// 2. Agent Config
	if provider != "" {
		env["RECAC_PROVIDER"] = provider
	}
	if model != "" {
		env["RECAC_MODEL"] = model
	}
	env["RECAC_PROJECT_ID"] = item.ID
	env["GIT_TERMINAL_PROMPT"] = "0"

	// 3. Git Identity
	env["GIT_AUTHOR_NAME"] = "RECAC Agent"
	env["GIT_AUTHOR_EMAIL"] = "agent@recac.io"
	env["GIT_COMMITTER_NAME"] = "RECAC Agent"
	env["GIT_COMMITTER_EMAIL"] = "agent@recac.io"

	// 4. Notifications
	if val := os.Getenv("RECAC_NOTIFICATIONS_DISCORD_ENABLED"); val != "" {
		env["RECAC_NOTIFICATIONS_DISCORD_ENABLED"] = val
	}
	if val := os.Getenv("RECAC_NOTIFICATIONS_SLACK_ENABLED"); val != "" {
		env["RECAC_NOTIFICATIONS_SLACK_ENABLED"] = val
	}

	// 5. Secrets
	secrets := []string{
		"JIRA_API_TOKEN", "JIRA_USERNAME", "JIRA_URL",
		"GITHUB_TOKEN", "GITHUB_API_KEY",
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "GEMINI_API_KEY", "OPENROUTER_API_KEY",
		"RECAC_DB_TYPE", "RECAC_DB_URL",
	}
	for _, secret := range secrets {
		if val := os.Getenv(secret); val != "" {
			env[secret] = val
			if secret == "GITHUB_API_KEY" {
				env["RECAC_GITHUB_API_KEY"] = val
			}
		}
	}

	// 6. Limits
	maxIterations := "20"
	if val := os.Getenv("RECAC_MAX_ITERATIONS"); val != "" {
		maxIterations = val
	}
	env["RECAC_MAX_ITERATIONS"] = maxIterations

	if val := os.Getenv("RECAC_MANAGER_FREQUENCY"); val != "" {
		env["RECAC_MANAGER_FREQUENCY"] = val
	}
	if val := os.Getenv("RECAC_TASK_MAX_ITERATIONS"); val != "" {
		env["RECAC_TASK_MAX_ITERATIONS"] = val
	}

	return env
}
