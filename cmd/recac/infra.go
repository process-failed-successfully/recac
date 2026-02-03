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

var (
	infraType     string
	infraProvider string
	infraDesc     string
	infraOut      string
	infraForce    bool
)

var infraCmd = &cobra.Command{
	Use:   "infra",
	Short: "Generate Infrastructure as Code (IaC)",
	Long: `Analyzes the project structure and uses AI to generate Infrastructure as Code (IaC) configuration.
Supports Terraform, Pulumi, CloudFormation, and Kubernetes Manifests.

Example:
  recac infra --type terraform --provider aws --desc "EKS cluster with VPC and RDS"
  recac infra --type pulumi --provider gcp --desc "Cloud Run service with Cloud SQL"
`,
	RunE: runInfra,
}

func init() {
	rootCmd.AddCommand(infraCmd)
	infraCmd.Flags().StringVar(&infraType, "type", "terraform", "IaC type (terraform, pulumi, cloudformation, k8s)")
	infraCmd.Flags().StringVar(&infraProvider, "provider", "", "Cloud provider (aws, gcp, azure, etc.)")
	infraCmd.Flags().StringVar(&infraDesc, "desc", "", "Description of the infrastructure to build")
	infraCmd.Flags().StringVarP(&infraOut, "out", "o", "infra", "Output directory")
	infraCmd.Flags().BoolVarP(&infraForce, "force", "f", false, "Overwrite existing files")
}

func runInfra(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Context (Project Structure)
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing project structure...")

	importantFiles := []string{
		"go.mod", "package.json", "requirements.txt", "pom.xml", "Dockerfile",
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("Project File Tree:\n")

	// Walk dir for structure
	err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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
			// Truncate if too huge
			runes := []rune(string(content))
			if len(runes) > 1000 {
				s := string(runes[:1000]) + "\n... (truncated)"
				contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f, s))
			} else {
				contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f, string(content)))
			}
		}
	}

	// 2. Prepare Prompt
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-infra")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert DevOps and Infrastructure Engineer.
Generate %s code for the %s provider.

Description: %s

Requirements:
1. Generate complete, valid, and production-ready code.
2. Separate concerns into multiple files if idiomatic (e.g., main.tf, variables.tf, outputs.tf for Terraform).
3. Do not assume previous state. Start from scratch.
4. Wrap the content of EACH file in XML tags like this:
<file path="main.tf">
... content ...
</file>

PROJECT CONTEXT:
%s
`, infraType, infraProvider, infraDesc, contextBuilder.String())

	// 3. Send to Agent
	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating %s configuration...\n", infraType)
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Parse Response using utils
	files := utils.ParseFileBlocks(resp)
	if len(files) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "❌ Could not parse files from agent response.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Raw Response:\n"+resp)
		return fmt.Errorf("failed to parse agent response")
	}

	// 5. Write Files
	if err := os.MkdirAll(infraOut, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for path, content := range files {
		// Prevent path traversal
		cleanPath := filepath.Clean(path)
		if strings.Contains(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || strings.HasPrefix(cleanPath, "\\") {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Skipping invalid path: %s\n", path)
			continue
		}

		fullPath := filepath.Join(infraOut, cleanPath)

		// Ensure the directory exists (for nested files)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for %s: %w", fullPath, err)
		}

		// Check overwrite
		if _, err := os.Stat(fullPath); err == nil && !infraForce {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Skipping %s (exists). Use --force to overwrite.\n", fullPath)
			continue
		}

		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fullPath, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Created %s\n", fullPath)
	}

	return nil
}
