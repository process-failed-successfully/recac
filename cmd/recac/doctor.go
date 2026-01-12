package main

import (
	"context"
	"fmt"
	"os"
	"time"
	"net/http"
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ErrSkipped is a special error to indicate that a check was intentionally skipped.
var ErrSkipped = errors.New("check skipped")

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a full diagnostic of the RECAC environment and configuration",
	Long: `The doctor command runs a comprehensive suite of checks to validate the
local environment, configuration, and connectivity to external services like
Docker, AI providers, and Jira.

It verifies:
  - Configuration file is present and valid.
  - Docker daemon is running and responsive.
  - AI provider API key is valid and can connect.
  - Jira credentials are correct (if configured).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🩺 Running RECAC Doctor...")
		allPassed := true

		runCheck("Configuration File", func() error {
			return checkConfig()
		}, &allPassed)

		runCheck("Go Binary", func() error {
			return checkGo()
		}, &allPassed)

		runCheck("Docker Daemon", func() error {
			return checkDocker()
		}, &allPassed)

		runCheck("AI Provider Connectivity", func() error {
			return checkAIProvider(cmd.Context(), nil)
		}, &allPassed)

		runCheck("Jira Connectivity", func() error {
			return checkJira(cmd.Context(), nil)
		}, &allPassed)

		fmt.Println()
		if allPassed {
			fmt.Println("✅ All checks passed! Your environment is ready. 🚀")
			return nil
		}

		fmt.Println("❌ Some checks failed. Please review the output above.")
		// Return an error to ensure the command exits with a non-zero status code.
		return fmt.Errorf("doctor command found issues")
	},
}

// runCheck is a helper to execute a diagnostic function and print its status.
func runCheck(title string, checkFunc func() error, allPassed *bool) {
	fmt.Printf("   - %-25s", title+":")
	err := checkFunc()
	switch {
	case err == nil:
		fmt.Println(" ✅ PASSED")
	case errors.Is(err, ErrSkipped):
		fmt.Printf(" ⚪ SKIPPED\n     Reason: %v\n", err)
	default:
		fmt.Printf(" ❌ FAILED\n     Error: %v\n", err)
		*allPassed = false
	}
}

// checkAIProvider verifies connectivity to the configured AI provider.
func checkAIProvider(ctx context.Context, transport http.RoundTripper) error {
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use a short timeout for the connectivity check.
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if transport != nil {
		ctx = context.WithValue(ctx, "transport", transport)
	}

	client, err := getAgentClient(ctx, provider, model, "", "doctor-check")
	if err != nil {
		return fmt.Errorf("failed to create agent client for provider '%s': %w", provider, err)
	}

	// Send a simple, harmless prompt to verify connectivity and authentication.
	// Using SendStream with a nil chunk handler is a lightweight way to do this.
	_, err = client.SendStream(ctx, "hello", func(s string) {})
	if err != nil {
		return fmt.Errorf("API key or connection for provider '%s' is invalid: %w", provider, err)
	}

	return nil
}

// checkJira verifies connectivity to the Jira instance if configured.
func checkJira(ctx context.Context, transport http.RoundTripper) error {
	// Only run the check if Jira URL is configured.
	if viper.GetString("jira.url") == "" && os.Getenv("JIRA_URL") == "" {
		return fmt.Errorf("%w: JIRA_URL not configured", ErrSkipped)
	}

	// Use a short timeout for the connectivity check.
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := getJiraClient(ctx)
	if err != nil {
		// We check for config presence above, so this failure is likely due to
		// missing username/token.
		return fmt.Errorf("failed to create Jira client: %w", err)
	}

	// If a mock transport is provided, use it.
	if transport != nil {
		client.HTTPClient.Transport = transport
	}

	if err := client.Authenticate(ctx); err != nil {
		return fmt.Errorf("Jira authentication failed: %w", err)
	}

	return nil
}
