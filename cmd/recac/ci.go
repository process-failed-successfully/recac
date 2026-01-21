package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ci",
		Short: "Generate CI/CD configuration files",
		Long: `Analyzes the project structure (languages, tools, frameworks) and generates an appropriate CI/CD configuration file using AI.

Supported platforms:
- github (GitHub Actions)
- gitlab (GitLab CI)
- circleci (CircleCI)
- jenkins (Jenkinsfile)
`,
		RunE: runCI,
	}

	cmd.Flags().StringP("platform", "p", "github", "Target CI platform (github, gitlab, circleci, jenkins)")
	cmd.Flags().StringP("output", "o", "", "Output file path (default depends on platform)")
	cmd.Flags().BoolP("force", "f", false, "Overwrite existing configuration file")

	return cmd
}

var ciCmd = NewCICmd()

func init() {
	rootCmd.AddCommand(ciCmd)
}

func runCI(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	ciPlatform, _ := cmd.Flags().GetString("platform")
	ciOutput, _ := cmd.Flags().GetString("output")
	ciForce, _ := cmd.Flags().GetBool("force")

	// 1. Determine Output Path
	if ciOutput == "" {
		switch strings.ToLower(ciPlatform) {
		case "github":
			ciOutput = ".github/workflows/ci.yml"
		case "gitlab":
			ciOutput = ".gitlab-ci.yml"
		case "circleci":
			ciOutput = ".circleci/config.yml"
		case "jenkins":
			ciOutput = "Jenkinsfile"
		default:
			return fmt.Errorf("unknown platform: %s", ciPlatform)
		}
	}

	// 2. Check existence
	if _, err := os.Stat(ciOutput); err == nil && !ciForce {
		return fmt.Errorf("configuration file '%s' already exists. Use --force to overwrite", ciOutput)
	}

	// 3. Generate Context (only structure and key files)
	// We don't need full content of all files, just structure and package manifests.
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing project structure...")

	// Scan specifically for build files to prioritize them
	importantFiles := []string{
		"go.mod", "go.sum",
		"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"requirements.txt", "Pipfile", "pyproject.toml",
		"pom.xml", "build.gradle",
		"Dockerfile", "docker-compose.yml",
		"Makefile",
	}

	// We'll use a custom context generation here to keep it lean
	var contextBuilder strings.Builder
	contextBuilder.WriteString("File Tree:\n")

	// Simple walk to get structure (ignoring .git, node_modules, etc.)
	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // ignore errors
		}
		rel, _ := filepath.Rel(cwd, path)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") || info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		contextBuilder.WriteString("- " + rel + "\n")
		return nil
	})
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to walk directory: %v\n", err)
	}

	contextBuilder.WriteString("\nKey Configuration Files:\n")
	for _, f := range importantFiles {
		if content, err := os.ReadFile(f); err == nil {
			// Truncate if too huge (rune-aware)
			runes := []rune(string(content))
			if len(runes) > 2000 {
				s := string(runes[:2000]) + "\n... (truncated)"
				contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f, s))
			} else {
				contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f, string(content)))
			}
		}
	}

	// 4. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-ci")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 5. Prompt
	prompt := fmt.Sprintf(`You are a DevOps expert.
Generate a complete, production-ready CI/CD configuration file for the following project.
Target Platform: %s

The configuration should:
- Run tests (if applicable)
- Lint code (if applicable)
- Build artifacts (if applicable)
- Use caching where possible to speed up builds

Output ONLY the raw content of the configuration file. Do not include markdown code blocks or explanations.

PROJECT CONTEXT:
%s`, ciPlatform, contextBuilder.String())

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating configuration...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Clean output
	content := utils.CleanCodeBlock(resp)

	// 6. Write File
	// Ensure dir exists
	dir := filepath.Dir(ciOutput)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if err := os.WriteFile(ciOutput, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ CI configuration written to %s\n", ciOutput)
	return nil
}
