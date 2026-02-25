package main

import (
	"fmt"
	"os"
	"recac/internal/analysis"
	"recac/internal/utils"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// analyzeDependenciesFunc is a package-level variable to allow mocking in tests.
var analyzeDependenciesFunc = analysis.AnalyzeDependencies

var (
	infraTarget string
	infraOutput string
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Generate infrastructure code (IaC) based on project dependencies",
	Long: `Analyzes the project's Go dependencies to identify required infrastructure services
(e.g., PostgreSQL, Redis, Kafka) and generates the corresponding configuration files.

Supported targets: docker-compose, k8s, terraform`,
	RunE: runInfra,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.Flags().StringVarP(&infraTarget, "target", "t", "docker-compose", "Target infrastructure type (docker-compose, k8s, terraform)")
	infraCmd.Flags().StringVarP(&infraOutput, "output", "o", "", "Output file path (default: stdout)")
}

func runInfra(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Analyze Dependencies
	fmt.Fprintf(cmd.ErrOrStderr(), "🔍 Analyzing dependencies in %s...\n", root)

	// We need module name to filter internal deps properly
	moduleName, err := analysis.GetModuleName(root)
	if err != nil {
		// Don't fail hard, just warn. Dependencies might still be valid.
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not determine module name: %v\n", err)
	}

	opts := analysis.DependencyOptions{
		Root:       root,
		ModuleName: moduleName,
		// We want external deps, so ignore nothing (except standard dirs handled by analysis)
		// But showStdLib=false to skip stdlib
		ShowStdLib: false,
	}

	deps, err := analyzeDependenciesFunc(opts)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// 2. Extract External Imports
	externalImports := extractExternalImports(deps, moduleName)
	if len(externalImports) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "No external dependencies found. Generating generic infrastructure.")
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "Found external dependencies: %v\n", externalImports)
	}

	// 3. Generate IaC
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-infra")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an Infrastructure as Code (IaC) expert.
Generate a %s configuration for a Go application with the following external dependencies:
%s

Requirements:
- The application service should be named "app".
- Include necessary environment variables for connecting to the detected services (e.g., DB_HOST, REDIS_URL).
- Use sensible defaults for versions and ports.
- Return ONLY the code for the configuration file. Do not include markdown blocks or explanations.
`, infraTarget, strings.Join(externalImports, "\n"))

	fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Generating %s configuration...\n", infraTarget)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	output := utils.CleanCodeBlock(resp)

	// 4. Output
	if infraOutput != "" {
		if err := os.WriteFile(infraOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Infrastructure code saved to %s\n", infraOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	return nil
}

func extractExternalImports(deps analysis.DepMap, moduleName string) []string {
	unique := make(map[string]bool)
	for _, targets := range deps {
		for _, tgt := range targets {
			// Skip internal packages
			if moduleName != "" && strings.HasPrefix(tgt, moduleName) {
				continue
			}
			unique[tgt] = true
		}
	}

	var result []string
	for k := range unique {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
