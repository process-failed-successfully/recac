package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"recac/internal/utils"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	apiTestOutput    string
	apiTestBaseURL   string
	apiTestFramework string
)

var apiTestCmd = &cobra.Command{
	Use:   "api-test [optional: base-url]",
	Short: "Generate integration tests for API routes",
	Long: `Scans the codebase for API routes and generates a standalone Go integration test file.
The generated tests verify status codes and basic response structures against a live server.

Example:
  recac api-test http://localhost:8080
  recac api-test --output e2e/api_test.go --framework testify
`,
	RunE: runApiTest,
}

func init() {
	rootCmd.AddCommand(apiTestCmd)
	apiTestCmd.Flags().StringVarP(&apiTestOutput, "output", "o", "e2e_test.go", "Output file for generated tests")
	apiTestCmd.Flags().StringVarP(&apiTestBaseURL, "base-url", "u", "", "Base URL for the API (e.g., http://localhost:8080)")
	apiTestCmd.Flags().StringVarP(&apiTestFramework, "framework", "f", "std", "Testing framework (std, testify)")
}

func runApiTest(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Determine Base URL
	baseURL := apiTestBaseURL
	if len(args) > 0 {
		baseURL = args[0]
	}

	// 1. Scan Routes
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning for API routes...")
	routes, err := analysis.ScanRoutes(cwd)
	if err != nil {
		return fmt.Errorf("failed to scan routes: %w", err)
	}

	if len(routes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No API routes found in the current directory.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d routes.\n", len(routes))

	// 2. Prepare AI Prompt
	var routeList strings.Builder
	for _, r := range routes {
		routeList.WriteString(fmt.Sprintf("- %s %s (Handler: %s)\n", r.Method, r.Path, r.Handler))
	}

	prompt := fmt.Sprintf(`You are an expert Go developer.
Generate a standalone Go integration test file (end-to-end tests) for the following API routes.

Routes:
%s

Requirements:
1. Use the "%s" testing framework.
2. The tests should target the base URL: "%s".
   - If the base URL is empty, assume it will be provided via an environment variable "API_BASE_URL" or default to "http://localhost:8080".
3. Include the build tag "//go:build e2e" at the top of the file so these tests don't run by default.
4. For each route, generate a test case that sends a request and checks the status code.
   - Infer expected status codes based on the method (e.g., POST -> 201, GET -> 200).
5. Use table-driven tests where appropriate to keep the code clean.
6. Return ONLY the raw Go code. Do not use Markdown formatting.

`, routeList.String(), apiTestFramework, baseURL)

	// 3. Call AI Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-api-test")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating integration tests...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Write Output
	content := utils.CleanCodeBlock(resp)

	// Ensure directory exists
	dir := filepath.Dir(apiTestOutput)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(apiTestOutput, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Tests generated in %s\n", apiTestOutput)
	fmt.Fprintln(cmd.OutOrStdout(), "\nTo run the tests:")
	if baseURL == "" {
		fmt.Printf("  API_BASE_URL=http://localhost:8080 go test -v -tags=e2e %s\n", apiTestOutput)
	} else {
		fmt.Printf("  go test -v -tags=e2e %s\n", apiTestOutput)
	}

	return nil
}
