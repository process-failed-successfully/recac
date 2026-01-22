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

// SetupGitWorkspace handles cloning, auth fallback, and Epic branching strategy
var SetupGitWorkspace = func(ctx context.Context, gitClient git.IClient, repoURL, workspace, ticketID, epicKey, timestamp string) (string, error) {
	if repoURL == "" {
		return "", nil // Nothing to clone
	}

	authRepoURL := handleAuth(repoURL)

	// Handle Git Ownership (Dubious ownership fix for Docker volumes)
	if workspace != "" {
		_ = gitClient.ConfigAddGlobal("safe.directory", workspace)
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

	configureGitIdentity(gitClient, workspace, ticketID)

	if epicKey != "" {
		handleEpicBranch(gitClient, workspace, ticketID, epicKey)
	}

	if err := checkoutFeatureBranch(gitClient, workspace, ticketID, timestamp); err != nil {
		return repoURL, err
	}

	return repoURL, nil
}

// SetupWorkspace is deprecated. Use SetupGitWorkspace instead.
var SetupWorkspace = SetupGitWorkspace

func handleAuth(repoURL string) string {
	githubKey := os.Getenv("GITHUB_API_KEY")
	if githubKey != "" && strings.Contains(repoURL, "github.com") && !strings.Contains(repoURL, "@") {
		return strings.Replace(repoURL, "https://github.com/", fmt.Sprintf("https://%s@github.com/", githubKey), 1)
	}
	return repoURL
}

func configureGitIdentity(gitClient git.IClient, workspace, ticketID string) {
	if err := gitClient.Config(workspace, "user.email", "agent@recac.com"); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to set git email: %v\n", ticketID, err)
	}
	if err := gitClient.Config(workspace, "user.name", "Recac Agent"); err != nil {
		fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to set git name: %v\n", ticketID, err)
	}
}

func handleEpicBranch(gitClient git.IClient, workspace, ticketID, epicKey string) {
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

func checkoutFeatureBranch(gitClient git.IClient, workspace, ticketID, timestamp string) error {
	uniqueNames := viper.GetBool("git.unique_branch_names")
	var branchName string
	if uniqueNames {
		branchName = fmt.Sprintf("agent/%s-%s", ticketID, timestamp)
	} else {
		branchName = fmt.Sprintf("agent/%s", ticketID)
	}

	fmt.Printf("[%s] preparing feature branch: %s\n", ticketID, branchName)

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
		if err := gitClient.Pull(workspace, "origin", branchName); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to pull branch: %v\n", ticketID, err)
		}
	} else {
		fmt.Printf("[%s] Creating and switching to new feature branch: %s\n", ticketID, branchName)
		if err := gitClient.CheckoutNewBranch(workspace, branchName); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to create branch: %v\n", ticketID, err)
		} else {
			fmt.Printf("[%s] Pushing branch to remote: %s\n", ticketID, branchName)
			if err := gitClient.Push(workspace, branchName); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] Warning: Failed to push branch: %v\n", ticketID, err)
			}
		}
	}
	return nil
}
