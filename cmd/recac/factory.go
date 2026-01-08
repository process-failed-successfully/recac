package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/git"
	"recac/internal/jira"
	"recac/internal/runner"
	"strings"

	"github.com/spf13/viper"
)

// ISessionManager defines the interface for session management to allow mocking.
type ISessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
	LoadSession(name string) (*runner.SessionState, error)
	IsProcessRunning(pid int) bool
	StartSession(name string, command []string, workspace string) (*runner.SessionState, error)
}

// newSessionManager is a factory function for creating a SessionManager.
// It's a variable so it can be replaced in tests.
var newSessionManager = func() (ISessionManager, error) {
	return runner.NewSessionManager()
}

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

// setupWorkspace handles cloning, auth fallback, and Epic branching strategy
func setupWorkspace(ctx context.Context, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
	if repoURL == "" {
		return "", nil // Nothing to clone
	}

	gitClient := git.NewClient()
	authRepoURL := repoURL

	// Handle Git Ownership (Dubious ownership fix for Docker volumes)
	if workspace != "" {
		_ = gitClient.ConfigAddGlobal("safe.directory", workspace)
	}

	// Handle GitHub Auth if token provided
	githubKey := os.Getenv("GITHUB_API_KEY")
	if githubKey != "" && strings.Contains(repoURL, "github.com") {
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
	if err := gitClient.Config(workspace, "user.email", "agent@recac.com"); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to set git email: %v\n", ticketID, err)
	}
	if err := gitClient.Config(workspace, "user.name", "Recac Agent"); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to set git name: %v\n", ticketID, err)
	}

	// Handle Epic Branching Strategy
	if epicKey != "" {
		epicBranch := fmt.Sprintf("agent-epic/%s", epicKey)
		fmt.Printf("[%s] Checking for Epic branch: %s\n", ticketID, epicBranch)

		exists, err := gitClient.RemoteBranchExists(workspace, "origin", epicBranch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to check remote for epic branch: %v\n", ticketID, err)
		}

		if exists {
			fmt.Printf("[%s] Epic branch '%s' found. Checking out...\n", ticketID, epicBranch)
			if err := gitClient.Fetch(workspace, "origin", epicBranch); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to fetch epic branch: %v\n", ticketID, err)
			}
			if err := gitClient.Checkout(workspace, epicBranch); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to checkout epic branch: %v\n", ticketID, err)
			}
		} else {
			fmt.Printf("[%s] Epic branch '%s' not found. Creating from default branch...\n", ticketID, epicBranch)
			if err := gitClient.CheckoutNewBranch(workspace, epicBranch); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to create epic branch: %v\n", ticketID, err)
			} else {
				if err := gitClient.Push(workspace, epicBranch); err != nil {
					fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to push epic branch: %v\n", ticketID, err)
				}
			}
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
	fmt.Printf("[%s] preparing feature branch: %s\n", ticketID, branchName)

	// Check if branch already exists remotely (for stable names)
	remoteExists, err := gitClient.RemoteBranchExists(workspace, "origin", branchName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to check remote for branch: %v\n", ticketID, err)
	}

	if remoteExists {
		fmt.Printf("[%s] Branch '%s' found remotely. Using existing branch.\n", ticketID, branchName)
		if err := gitClient.Fetch(workspace, "origin", branchName); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to fetch branch: %v\n", ticketID, err)
		}
		if err := gitClient.Checkout(workspace, branchName); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to checkout branch: %v\n", ticketID, err)
		}
		// Pull latest changes to be sure (rebase preferred strictly but merge ok for agent)
		if err := gitClient.Pull(workspace, "origin", branchName); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to pull branch: %v\n", ticketID, err)
		}
	} else {
		// New Branch
		fmt.Printf("[%s] Creating and switching to new feature branch: %s\n", ticketID, branchName)
		if err := gitClient.CheckoutNewBranch(workspace, branchName); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to create branch: %v\n", ticketID, err)
		} else {
			// Push the branch immediately
			fmt.Printf("[%s] Pushing branch to remote: %s\n", ticketID, branchName)
			if err := gitClient.Push(workspace, branchName); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to push branch: %v\n", ticketID, err)
			}
		}
	}

	return repoURL, nil
}
