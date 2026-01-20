package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var refactorCmd = &cobra.Command{
	Use:   "refactor [files...]",
	Short: "Refactor multiple files using AI",
	Long: `Refactor one or more files based on a natural language instruction.
This command is context-aware and can handle multi-file refactorings (e.g., renaming a struct used in multiple files).

Example:
  recac refactor pkg/models/user.go pkg/services/user_service.go -p "Rename User.Name to User.FullName and update usages"
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRefactor,
}

func init() {
	rootCmd.AddCommand(refactorCmd)
	refactorCmd.Flags().StringP("prompt", "p", "", "Refactoring instruction (required)")
	refactorCmd.MarkFlagRequired("prompt")
	refactorCmd.Flags().Bool("diff", false, "Show diff between original and refactored code")
	refactorCmd.Flags().BoolP("in-place", "i", false, "Modify the files in place")
}

func runRefactor(cmd *cobra.Command, args []string) error {
	prompt, _ := cmd.Flags().GetString("prompt")
	showDiff, _ := cmd.Flags().GetBool("diff")
	inPlace, _ := cmd.Flags().GetBool("in-place")

	// 1. Read all files
	fileContents := make(map[string]string)
	var promptBuilder strings.Builder

	promptBuilder.WriteString("Instruction: " + prompt + "\n\n")
	promptBuilder.WriteString("Refactor the following files as requested. Return the modified content for ALL files that need changes.\n")
	promptBuilder.WriteString("IMPORTANT: You MUST return the code wrapped in XML-like tags as follows:\n")
	promptBuilder.WriteString(`<file path="path/to/file.go">
... code ...
</file>
`)
	promptBuilder.WriteString("Only return files that have been modified. If a file does not need changes, do not include it in the output.\n\n")

	for _, path := range args {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}
		fileContents[path] = string(content)

		promptBuilder.WriteString(fmt.Sprintf("<file path=\"%s\">\n%s\n</file>\n\n", path, string(content)))
	}

	// 2. Initialize Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-refactor")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Analyzing and refactoring %d files...\n", len(args))

	// 3. Send to Agent
	resp, err := ag.Send(ctx, promptBuilder.String())
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 4. Parse Response
	// The agent might wrap the whole thing in markdown, so we might need to clean that first?
	// But ParseFileBlocks is robust to surrounding text.
	modifiedFiles := utils.ParseFileBlocks(resp)

	if len(modifiedFiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes proposed by the agent.")
		return nil
	}

	// 5. Handle Output
	for path, newContent := range modifiedFiles {
		originalContent, exists := fileContents[path]
		if !exists {
			// Agent might have halluncinated a path or renamed one.
			// For safety, we only allow modifying existing files passed as args.
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Skipping file %s (not in input arguments)\n", path)
			continue
		}

		if showDiff {
			diff, err := utils.GenerateDiff(path, originalContent, newContent)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to generate diff for %s: %v\n", path, err)
			} else {
				fmt.Fprint(cmd.OutOrStdout(), diff)
			}
		}

		if inPlace {
			// Ensure directory exists if it's a new path
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}

			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return fmt.Errorf("failed to write %s: %w", path, err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Updated %s\n", path)
		}
	}

	if !inPlace && !showDiff {
		// If neither flag, print the raw output or a summary?
		// Printing raw file blocks is messy.
		// Let's print a summary and instructions.
		fmt.Fprintln(cmd.OutOrStdout(), "\nProposed changes:")
		for path := range modifiedFiles {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", path)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nRun with --diff to see changes, or --in-place to apply them.")
	}

	return nil
}
