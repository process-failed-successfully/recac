package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	helmChartName string
	helmOutputDir string
	helmPort      string
	helmForce     bool
)

var helmCmd = &cobra.Command{
	Use:   "helm",
	Short: "Generate a Helm Chart for the project",
	Long: `Analyzes the project structure (Dockerfile, code) and uses AI to generate
a production-ready Helm Chart, including Chart.yaml, values.yaml, and templates.

It automatically detects the application's requirements (ports, env vars) and
creates a standard Helm chart structure.`,
	RunE: runHelm,
}

func init() {
	rootCmd.AddCommand(helmCmd)
	helmCmd.Flags().StringVarP(&helmChartName, "name", "n", "", "Name of the chart (default: current directory name)")
	helmCmd.Flags().StringVarP(&helmOutputDir, "output", "o", "", "Output directory (default: charts/<name>)")
	helmCmd.Flags().StringVar(&helmPort, "port", "", "Service port to expose (e.g., 8080)")
	helmCmd.Flags().BoolVarP(&helmForce, "force", "f", false, "Overwrite existing files")
}

func runHelm(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Determine Chart Name and Output Dir
	if helmChartName == "" {
		helmChartName = filepath.Base(cwd)
	}

	if helmOutputDir == "" {
		helmOutputDir = filepath.Join("charts", helmChartName)
	}

	// 2. Gather Context
	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing project for chart '%s'...\n", helmChartName)

	importantFiles := []string{
		"Dockerfile",
		"docker-compose.yml",
		"go.mod",
		"package.json",
		"requirements.txt",
		"pom.xml",
	}

	var contextBuilder strings.Builder
	contextBuilder.WriteString("File Tree:\n")

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
			if len(runes) > 2000 {
				s := string(runes[:2000]) + "\n... (truncated)"
				contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f, s))
			} else {
				contextBuilder.WriteString(fmt.Sprintf("--- %s ---\n%s\n\n", f, string(content)))
			}
		}
	}

	// 3. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-helm")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Kubernetes and Helm expert.
Generate a complete Helm Chart for the following project.
Chart Name: %s

Files required:
1. Chart.yaml (apiVersion: v2)
2. values.yaml (Default configuration)
3. templates/deployment.yaml
4. templates/service.yaml
5. templates/ingress.yaml
6. templates/_helpers.tpl

Configuration:
`, helmChartName)

	if helmPort != "" {
		prompt += fmt.Sprintf("- Service Port: %s\n", helmPort)
	} else {
		prompt += "- Detect appropriate port from context if possible, otherwise default to 80.\n"
	}

	prompt += `
IMPORTANT: Output the content of each file wrapped in XML tags like this:
<file path="Chart.yaml">
... content ...
</file>
<file path="templates/deployment.yaml">
... content ...
</file>

Do not use markdown code blocks inside the XML tags.
Output ONLY the XML structure.

PROJECT CONTEXT:
` + contextBuilder.String()

	// 4. Send to Agent
	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating Helm Chart...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Parse Response
	files := parseXMLFilesHelm(resp)
	if len(files) == 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "❌ Could not parse files from agent response.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Raw Response:\n"+resp)
		return fmt.Errorf("failed to parse agent response")
	}

	// 6. Write Files
	if err := os.MkdirAll(helmOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for path, content := range files {
		fullPath := filepath.Join(helmOutputDir, path)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}

		// Check overwrite
		if _, err := os.Stat(fullPath); err == nil && !helmForce {
			fmt.Fprintf(cmd.ErrOrStderr(), "⚠️  Skipping %s (exists). Use --force to overwrite.\n", fullPath)
			continue
		}

		// Clean content (trim whitespace)
		content = strings.TrimSpace(content)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", fullPath, err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Created %s\n", fullPath)
	}

	return nil
}

func parseXMLFilesHelm(text string) map[string]string {
	result := make(map[string]string)
	// Regex to match <file path="...">...</file>
	// (?s) enables dot matching newline
	re := regexp.MustCompile(`(?s)<file\s+path="([^"]+)">\s*(.*?)\s*</file>`)
	matches := re.FindAllStringSubmatch(text, -1)

	for _, match := range matches {
		if len(match) == 3 {
			path := match[1]
			content := match[2]
			result[path] = content
		}
	}
	return result
}
