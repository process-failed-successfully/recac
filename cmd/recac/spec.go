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

var specOutput string
var specExclude []string

var specCmd = &cobra.Command{
	Use:   "spec",
	Short: "Generate app_spec.txt from existing codebase",
	Long:  `Analyzes the current directory and uses the AI agent to generate a comprehensive application specification (app_spec.txt).`,
	RunE:  runSpec,
}

func runSpec(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Analyzing codebase...")
	contextStr, err := collectProjectContext(cwd, specExclude)
	if err != nil {
		return fmt.Errorf("failed to collect project context: %w", err)
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	projectName := filepath.Base(cwd)

	ag, err := agentClientFactory(ctx, provider, model, cwd, projectName)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following codebase structure and content.
Create a comprehensive application specification (app_spec.txt) that describes:
1. The project's purpose and high-level architecture.
2. Key features and functionality.
3. Technical stack and dependencies.
4. Directory structure overview.

Output ONLY the content of the app_spec.txt.

Codebase Context:
%s
`, contextStr)

	fmt.Fprintln(cmd.OutOrStdout(), "Generating spec...")

	var outputBuilder strings.Builder
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
		outputBuilder.WriteString(chunk)
	})
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Ensure final newline
	fmt.Fprintln(cmd.OutOrStdout(), "")

	if specOutput != "" {
		err = os.WriteFile(specOutput, []byte(outputBuilder.String()), 0644)
		if err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Spec saved to %s\n", specOutput)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(specCmd)
	specCmd.Flags().StringVarP(&specOutput, "output", "o", "app_spec.txt", "Output file path")
	specCmd.Flags().StringSliceVarP(&specExclude, "exclude", "e", []string{}, "Glob patterns to exclude")
}

func collectProjectContext(root string, extraExcludes []string) (string, error) {
	var sb strings.Builder

	ignoredDirs := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		".recac":       true,
		".idea":        true,
		".vscode":      true,
		"coverage":     true,
	}

	maxTotalSize := 200 * 1024 // 200KB limit for context
	currentSize := 0
	skippedCount := 0
	maxSkippedLogged := 50

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if ignoredDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extra excludes
		for _, pattern := range extraExcludes {
			matched, _ := filepath.Match(pattern, info.Name())
			if matched {
				return nil
			}
		}

		// Skip binary files and likely large files (simple check)
		if info.Size() > 50*1024 { // Skip files > 50KB individual
			sb.WriteString(fmt.Sprintf("File: %s (Skipped: too large)\n", relPath))
			return nil
		}

		// Skip if total size limit reached, but still list file
		if currentSize >= maxTotalSize {
			skippedCount++
			if skippedCount <= maxSkippedLogged {
				sb.WriteString(fmt.Sprintf("File: %s (Skipped: context limit reached)\n", relPath))
			} else if skippedCount == maxSkippedLogged+1 {
				sb.WriteString("... (remaining files skipped due to context limit)\n")
			}
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			// Skip files we can't read
			return nil
		}

		// Basic binary check (look for null byte in first 512 bytes)
		isBinary := false
		checkLen := 512
		if len(content) < 512 {
			checkLen = len(content)
		}
		for i := 0; i < checkLen; i++ {
			if content[i] == 0 {
				isBinary = true
				break
			}
		}

		if isBinary {
			sb.WriteString(fmt.Sprintf("File: %s (Binary)\n", relPath))
			return nil
		}

		sb.WriteString(fmt.Sprintf("-- %s --\n", relPath))
		sb.WriteString(string(content))
		sb.WriteString("\n\n")

		currentSize += len(content)

		return nil
	})

	return sb.String(), err
}
