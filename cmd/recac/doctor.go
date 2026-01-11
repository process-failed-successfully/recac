package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"context"
	"recac/internal/config"
	"recac/internal/docker"
	"recac/internal/jira"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var (
	runCheckJira         = checkJira
	runCheckAIProvider   = checkAIProvider
	runCheckDocker       = checkDockerServiceAvailability
	runCheckOrchestrator = checkOrchestrator
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a series of checks to diagnose issues with your recac setup",
	Long:  `The doctor command runs a suite of checks to validate the local configuration, connectivity to external services, and the status of core components.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		failedChecks := 0
		cmd.Println("🩺 Running recac doctor...")

		check(cmd, "Configuration", validateConfig, &failedChecks)
		check(cmd, "AI Provider Connectivity", runCheckAIProvider, &failedChecks)
		check(cmd, "Jira Connectivity", runCheckJira, &failedChecks)
		check(cmd, "Docker Service", runCheckDocker, &failedChecks)
		check(cmd, "Orchestrator Status", runCheckOrchestrator, &failedChecks)

		if failedChecks > 0 {
			cmd.Printf("\n✖️ Doctor found %d issue(s).\n", failedChecks)
			return fmt.Errorf("%d checks failed", failedChecks)
		}

		cmd.Println("\n✅ All checks passed!")
		return nil
	},
}

// check is a helper to run a diagnostic function and print the outcome.
func check(cmd *cobra.Command, title string, checkFunc func() error, errorCount *int) {
	cmd.Printf("Checking %s...", title)
	if err := checkFunc(); err != nil {
		cmd.Println(" ✖️")
		cmd.Printf("  Error: %v\n", err)
		(*errorCount)++
	} else {
		cmd.Println(" ✓")
	}
}

// validateConfig checks if the essential configuration is present.
func validateConfig() error {
	// We need to trigger the config loading manually since the doctor command
	// doesn't automatically run the root `initConfig`.
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// It's okay if the config file doesn't exist, but we must check env vars.
		} else {
			return fmt.Errorf("could not read config file: %w", err)
		}
	}
	// Call the global validator
	if err := config.ValidateConfig(); err != nil {
		return err
	}
	// Add specific checks for doctor
	if viper.GetString("agent_provider") == "" {
		return fmt.Errorf("`agent_provider` is not set in config or RECAC_AGENT_PROVIDER env var")
	}
	if viper.GetString("agent_model") == "" {
		return fmt.Errorf("`agent_model` is not set in config or RECAC_AGENT_MODEL env var")
	}
	if viper.GetString("api_key") == "" && os.Getenv("GEMINI_API_KEY") == "" && os.Getenv("OPENAI_API_KEY") == "" {
		return fmt.Errorf("`api_key` is not set and no provider-specific API key (e.g., GEMINI_API_KEY) is found")
	}
	return nil
}

// checkJira verifies connectivity and authentication with Jira.
func checkJira() error {
	jiraURL := viper.GetString("jira.url")
	jiraEmail := viper.GetString("jira.email")
	jiraToken := viper.GetString("jira.token")

	if jiraURL == "" {
		// Jira not configured, skip the check.
		return nil
	}

	if jiraEmail == "" || jiraToken == "" {
		return fmt.Errorf("Jira is partially configured. `jira.email` and `jira.token` are required if `jira.url` is set")
	}

	client := jira.NewClient(jiraURL, jiraEmail, jiraToken)
	if err := client.Authenticate(context.Background()); err != nil {
		return fmt.Errorf("Jira authentication failed: %w", err)
	}

	return nil
}

func checkDockerServiceAvailability() error {
	cli, err := docker.NewClient("")
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	_, err = cli.ServerVersion(context.Background())
	if err != nil {
		return fmt.Errorf("could not connect to Docker daemon: %w", err)
	}
	return nil
}

func checkOrchestrator() error {
	sm, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("could not create session manager: %w", err)
	}
	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("could not list sessions: %w", err)
	}
	for _, s := range sessions {
		if strings.Contains(s.Name, "orchestrator") && s.Status == "RUNNING" {
			return nil // An orchestrator is running
		}
	}
	// No running orchestrator found, which is not an error, just a state.
	// We can return a specific message if we want, but for now, nil is fine.
	return nil
}

// checkAIProvider verifies connectivity to the configured AI provider.
func checkAIProvider() error {
	provider := viper.GetString("agent_provider")
	apiKey := viper.GetString("api_key")

	switch provider {
	case "gemini":
		if apiKey == "" {
			apiKey = os.Getenv("GEMINI_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("GEMINI_API_KEY not found for gemini provider")
		}
		return checkGemini(apiKey)
	case "openai":
		if apiKey == "" {
			apiKey = os.Getenv("OPENAI_API_KEY")
		}
		if apiKey == "" {
			return fmt.Errorf("OPENAI_API_KEY not found for openai provider")
		}
		return fmt.Errorf("openai check not yet implemented")
	case "openrouter":
		return fmt.Errorf("openrouter check not yet implemented")
	case "ollama":
		return fmt.Errorf("ollama check not yet implemented")
	default:
		return fmt.Errorf("unknown agent provider: %s", provider)
	}
}

func checkGemini(apiKey string) error {
	url := "https://generativelanguage.googleapis.com/v1beta/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("x-goog-api-key", apiKey)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned non-200 status: %d - %s", resp.StatusCode, string(body))
	}

	return nil
}
