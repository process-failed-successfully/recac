package orchestrator

import (
	"fmt"
	"os"
	"sort"

	"github.com/kballard/go-shellquote"
)

// Shared secrets list to ensure consistency between Docker and K8s spawners
var Secrets = []string{
	"JIRA_API_TOKEN",
	"JIRA_USERNAME",
	"JIRA_URL",
	"GITHUB_TOKEN",
	"GITHUB_API_KEY",
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
	"GEMINI_API_KEY",
	"OPENROUTER_API_KEY",
	"RECAC_DB_TYPE",
	"RECAC_DB_URL",
}

// BuildEnvExports constructs a deterministic list of export commands for environment variables.
func BuildEnvExports(envVars map[string]string) []string {
	var exports []string

	// Sort keys for deterministic output
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := envVars[k]
		exports = append(exports, fmt.Sprintf("export %s=%s", k, shellquote.Join(v)))
	}
	return exports
}

// BuildSecretExports constructs export commands for known secrets from the host environment.
func BuildSecretExports() []string {
	var exports []string
	for _, secret := range Secrets {
		if val := os.Getenv(secret); val != "" {
			quotedVal := shellquote.Join(val)
			exports = append(exports, fmt.Sprintf("export %s=%s", secret, quotedVal))
			if secret == "GITHUB_API_KEY" {
				exports = append(exports, fmt.Sprintf("export RECAC_GITHUB_API_KEY=%s", quotedVal))
			}
		}
	}
	return exports
}
