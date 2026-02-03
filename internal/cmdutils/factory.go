package cmdutils

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/git"
	"recac/internal/jira"
	"strings"

	"github.com/spf13/viper"
)

// GetJiraClient initializes a Jira client using config or environment variables
var GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
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

// GetAgentClient initializes an Agent client based on provider and configuration
var GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
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
	if apiKey == "" && provider != "ollama" && provider != "gemini-cli" && provider != "cursor-cli" && provider != "opencode" {
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

// SetupWorkspace handles cloning, auth fallback, and Epic branching strategy
var SetupWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
	if repoURL == "" {
		return "", nil // Nothing to clone
	}

	authRepoURL := repoURL

	// Handle Git Ownership (Dubious ownership fix for Docker volumes)
	if workspace != "" {
		_ = gitClient.ConfigAddGlobal("safe.directory", workspace)
	}

	// Handle GitHub Auth if token provided
	githubKey := os.Getenv("GITHUB_API_KEY")
	if githubKey != "" && strings.Contains(repoURL, "github.com") && !strings.Contains(repoURL, "@") {
		authRepoURL = strings.Replace(repoURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", githubKey), 1)
	}

	// 2. Clone Repository (if not already present)
	if !gitClient.RepoExists(workspace) {
		fmt.Printf("[%s] Cloning repository into %s...\n", ticketID, workspace)
		if err := gitClient.Clone(ctx, authRepoURL, workspace); err != nil {
			return repoURL, fmt.Errorf("failed to clone repository: %w", err)
		}
	} else {
		fmt.Printf("[%s] Repository already exists in %s, skipping clone.\n", ticketID, workspace)
	}

	// Configure Git Identity for Agent
	if err := gitClient.ConfigureIdentity(workspace, "Recac Agent", "agent@recac.com"); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to configure git identity: %v\n", ticketID, err)
	}

	// Handle Epic Branching Strategy
	if epicKey != "" {
		epicBranch := fmt.Sprintf("agent-epic/%s", epicKey)
		fmt.Printf("[%s] Syncing Epic branch: %s\n", ticketID, epicBranch)

		if err := gitClient.SyncBranch(ctx, workspace, epicBranch, fmt.Sprintf("[%s] ", ticketID)); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to sync epic branch: %v\n", ticketID, err)
		}
	}

	// Determine Branch Name
	uniqueNames := viper.GetBool("git.unique_branch_names")
	var branchName string
	if uniqueNames {
		branchName = fmt.Sprintf("agent/%s-%s", ticketID, timestamp)
	} else {
		branchName = fmt.Sprintf("agent/%s", ticketID)
	}

	// Create and Checkout Feature Branch
	fmt.Printf("[%s] Syncing feature branch: %s\n", ticketID, branchName)

	if err := gitClient.SyncBranch(ctx, workspace, branchName, fmt.Sprintf("[%s] ", ticketID)); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to sync feature branch: %v\n", ticketID, err)
	}

	return repoURL, nil
}
