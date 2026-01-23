package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var readmeCmd = &cobra.Command{
	Use:   "readme [path]",
	Short: "Generate a README.md for the project using AI",
	Long: `Scans the project structure and key files to generate a comprehensive README.md file.
It automatically detects the language, dependencies, and entry points.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReadme,
}

func init() {
	readmeCmd.Flags().StringP("out", "o", "README.md", "Output file path")
	rootCmd.AddCommand(readmeCmd)
}

func runReadme(cmd *cobra.Command, args []string) error {
	projectPath := "."
	if len(args) > 0 {
		projectPath = args[0]
	}

	outFile, _ := cmd.Flags().GetString("out")

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Scanning project at %s...\n", projectPath)

	contextStr, err := collectReadmeContext(projectPath)
	if err != nil {
		return fmt.Errorf("failed to scan project: %w", err)
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use absolute path for agent context
	absPath, _ := filepath.Abs(projectPath)

	ag, err := agentClientFactory(ctx, provider, model, absPath, "recac-readme")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert technical writer.
Generate a comprehensive README.md for the following project.
The README should include:
- Project Title & Description
- Key Features
- Installation Instructions
- Usage Examples
- Configuration
- Contributing Guidelines
- License

Project Context:
%s

Output ONLY the Markdown content for the README.md.`, contextStr)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating README...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate readme: %w", err)
	}

	// Clean up response if it contains markdown code blocks
	cleanResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanResp, "```markdown") {
		cleanResp = strings.TrimPrefix(cleanResp, "```markdown")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	} else if strings.HasPrefix(cleanResp, "```md") {
		cleanResp = strings.TrimPrefix(cleanResp, "```md")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	} else if strings.HasPrefix(cleanResp, "```") {
		cleanResp = strings.TrimPrefix(cleanResp, "```")
		cleanResp = strings.TrimSuffix(cleanResp, "```")
	}
	cleanResp = strings.TrimSpace(cleanResp)

	if err := os.WriteFile(outFile, []byte(cleanResp), 0644); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ README generated at %s\n", outFile)
	return nil
}

func collectReadmeContext(root string) (string, error) {
	var sb strings.Builder
	var structure strings.Builder

	ignoreMap := DefaultIgnoreMap()

	// Priority files to include content for
	priorityFiles := map[string]bool{
		"go.mod":           true,
		"package.json":     true,
		"requirements.txt": true,
		"pom.xml":          true,
		"Makefile":         true,
		"Dockerfile":       true,
		"main.go":          true,
		"index.js":         true,
		"app.py":           true,
		"Cargo.toml":       true,
		"CONTRIBUTING.md":  true,
	}

	// Limit total context size (approx chars)
	const maxContextSize = 50000
	currentSize := 0

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		relPath, _ := filepath.Rel(root, path)
		if relPath == "." {
			return nil
		}

		// Check ignore map
		parts := strings.Split(relPath, string(os.PathSeparator))
		for _, part := range parts {
			if ignoreMap[part] {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Add to structure tree
		depth := strings.Count(relPath, string(os.PathSeparator))
		indent := strings.Repeat("  ", depth)
		structure.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))

		// Check if it's a priority file
		if !info.IsDir() && priorityFiles[info.Name()] {
			content, err := os.ReadFile(path)
			if err == nil {
				// Truncate if too large
				strContent := string(content)
				if len(strContent) > 5000 {
					strContent = strContent[:5000] + "\n... (truncated)"
				}

				formatted := fmt.Sprintf("\nFile: %s\n```\n%s\n```\n", relPath, strContent)
				if currentSize+len(formatted) < maxContextSize {
					sb.WriteString(formatted)
					currentSize += len(formatted)
				}
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Directory Structure:\n%s\nKey Files Content:\n%s", structure.String(), sb.String()), nil
}
