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
	contOutputDir string
	contCompose   bool
	contPort      string
	contDB        string
	contForce     bool
)

var containerizeCmd = &cobra.Command{
	Use:     "containerize",
	Aliases: []string{"dockerize"},
	Short:   "Generate Dockerfile and docker-compose.yml",
	Long: `Analyzes the project structure and uses AI to generate a production-ready
Dockerfile, .dockerignore, and optional docker-compose.yml.

It detects languages and frameworks (Node.js, Go, Python, etc.) and suggests
best practices for multi-stage builds and image optimization.`,
	RunE: runContainerize,
}

func init() {
	rootCmd.AddCommand(containerizeCmd)
	containerizeCmd.Flags().StringVarP(&contOutputDir, "output-dir", "o", ".", "Directory to write generated files")
	containerizeCmd.Flags().BoolVar(&contCompose, "compose", true, "Generate docker-compose.yml")
	containerizeCmd.Flags().StringVar(&contPort, "port", "", "Port to expose (e.g. 8080)")
	containerizeCmd.Flags().StringVar(&contDB, "db", "", "Database service to include (e.g. postgres, mysql, mongo, redis)")
	containerizeCmd.Flags().BoolVarP(&contForce, "force", "f", false, "Overwrite existing files")
}

func runContainerize(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Gather Context
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing project structure...")

	importantFiles := []string{
		"go.mod", "go.sum",
		"package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"requirements.txt", "Pipfile", "pyproject.toml",
		"pom.xml", "build.gradle",
		"Gemfile", "Gemfile.lock",
		"composer.json",
		"Cargo.toml",
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

	// 2. Prepare Prompt
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-containerize")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a Docker and DevOps expert.
Generate the following files for this project to containerize it:
1. Dockerfile (Production ready, multi-stage if applicable)
2. .dockerignore
`)

	if contCompose {
		prompt += "3. docker-compose.yml\n"
	}

	prompt += "\nConfiguration:\n"
	if contPort != "" {
		prompt += fmt.Sprintf("- Expose port: %s\n", contPort)
	}
	if contDB != "" {
		prompt += fmt.Sprintf("- Include database service: %s (in docker-compose.yml)\n", contDB)
	}

	prompt += `
IMPORTANT: Output the content of each file wrapped in XML tags like this:
<file path="Dockerfile">
... content ...
</file>
<file path=".dockerignore">
... content ...
</file>

Do not use markdown code blocks inside the XML tags if possible, or make sure the XML tags are outside them.
Output ONLY the XML structure.

PROJECT CONTEXT:
` + contextBuilder.String()

	// 3. Send to Agent
	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating Docker assets...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Parse Response
	files := parseXMLFiles(resp)
	if len(files) == 0 {
		// Fallback: maybe the agent didn't use XML tags?
		// If response contains "Dockerfile", maybe we can just dump it to a file?
		// But it's risky. Let's error for now or print raw.
		fmt.Fprintln(cmd.ErrOrStderr(), "❌ Could not parse files from agent response.")
		fmt.Fprintln(cmd.ErrOrStderr(), "Raw Response:\n"+resp)
		return fmt.Errorf("failed to parse agent response")
	}

	// 5. Write Files
	if err := os.MkdirAll(contOutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	for path, content := range files {
		fullPath := filepath.Join(contOutputDir, path)

		// Check overwrite
		if _, err := os.Stat(fullPath); err == nil && !contForce {
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

func parseXMLFiles(text string) map[string]string {
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
