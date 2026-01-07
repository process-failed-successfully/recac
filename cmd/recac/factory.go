package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/jira"
	"recac/internal/runner"

	"github.com/spf13/viper"
)

// newSessionManager is a variable to allow dependency injection for testing.
var newSessionManager = runner.NewSessionManager

// newDockerClient is a variable to allow dependency injection for testing.
var newDockerClient = func(projectName string) (docker.APIClient, error) {
	return docker.NewClient(projectName)
}

// getJiraClient creates a new Jira client from config.
func getJiraClient(ctx context.Context) (*jira.Client, error) {
	// Viper precedence: flag > env > config file > default
	// Env vars are automatically read by Viper if they match JIRA_URL etc.
	jiraURL := viper.GetString("jira.url")
	if jiraURL == "" {
		return nil, fmt.Errorf("jira URL not set. Use --jira-url flag, JIRA_URL env var, or jira.url in config")
	}

	jiraUsername := viper.GetString("jira.username")
	if jiraUsername == "" {
		return nil, fmt.Errorf("jira username not set. Use --jira-username flag, JIRA_USERNAME env var, or jira.username in config")
	}

	// API token is sensitive, so we prioritize env var
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	if jiraToken == "" {
		jiraToken = viper.GetString("jira.api_token")
	}
	if jiraToken == "" {
		return nil, fmt.Errorf("jira API token not set. Use JIRA_API_TOKEN env var or jira.api_token in config")
	}

	return jira.NewClient(jiraURL, jiraUsername, jiraToken), nil
}

// getAgentClient creates a new agent client from config.
func getAgentClient(ctx context.Context, provider, model, workspace, project string) (agent.Agent, error) {
	// Use provided provider/model if available, otherwise fallback to config
	if provider == "" {
		provider = viper.GetString("provider")
	}
	if model == "" {
		model = viper.GetString("model")
	}

	// API key is sensitive, so we prioritize env var
	apiKey := os.Getenv(agent.ProviderToEnvVar(provider))
	if apiKey == "" {
		apiKey = viper.GetString("api_key")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("API key not set for provider %s. Use %s env var or api_key in config", provider, agent.ProviderToEnvVar(provider))
	}

	return agent.NewAgent(provider, apiKey, model, workspace, project)
}
