package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/jira"

	"github.com/spf13/viper"
)

// getJiraClient initializes a Jira client using config or environment variables
func getJiraClient(ctx context.Context) (*jira.Client, error) {
	baseURL := viper.GetString("jira.url")
	username := viper.GetString("jira.username")
	apiToken := viper.GetString("jira.api_token")

	// Fallback to environment variables
	if baseURL == "" {
		baseURL = os.Getenv("JIRA_URL")
	}
	if username == "" {
		username = os.Getenv("JIRA_USERNAME")
		if username == "" {
			username = os.Getenv("JIRA_EMAIL")
		}
	}
	if apiToken == "" {
		apiToken = os.Getenv("JIRA_API_TOKEN")
	}

	// Validate required fields
	if baseURL == "" {
		return nil, fmt.Errorf("JIRA_URL environment variable or jira.url config is required")
	}
	if username == "" {
		return nil, fmt.Errorf("JIRA_USERNAME environment variable or jira.username config is required")
	}
	if apiToken == "" {
		return nil, fmt.Errorf("JIRA_API_TOKEN environment variable or jira.api_token config is required")
	}

	return jira.NewClient(baseURL, username, apiToken), nil
}

// getAgentClient initializes an Agent client based on provider and configuration
func getAgentClient(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
	if provider == "" {
		provider = viper.GetString("provider")
		if provider == "" {
			provider = "gemini"
		}
	}

	apiKey := viper.GetString("api_key")
	if apiKey == "" {
		apiKey = os.Getenv("API_KEY")
		if apiKey == "" {
			switch provider {
			case "gemini":
				apiKey = os.Getenv("GEMINI_API_KEY")
			case "openai":
				apiKey = os.Getenv("OPENAI_API_KEY")
			case "openrouter":
				apiKey = os.Getenv("OPENROUTER_API_KEY")
			}
		}
	}

	// Final fallback for developers or testing if not ollama
	if apiKey == "" && provider != "ollama" && provider != "gemini-cli" && provider != "cursor-cli" {
		apiKey = "dummy-key"
	}

	if model == "" {
		model = viper.GetString("model")
		if model == "" {
			switch provider {
			case "openrouter":
				model = "deepseek/deepseek-v3.2"
			case "gemini":
				model = "gemini-pro"
			case "openai":
				model = "gpt-4"
			}
		}
	}

	return agent.NewAgent(provider, apiKey, model, projectPath, projectName)
}
