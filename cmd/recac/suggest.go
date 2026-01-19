package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	suggestType   string
	suggestLimit  int
	suggestIgnore []string
	suggestFocus  string
)

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Proactively suggest improvements using AI",
	Long: `Analyzes the codebase and suggests actionable improvements, bugs to fix, or refactoring opportunities.
Found suggestions can be interactively added to your TODO list.`,
	RunE: runSuggest,
}

func init() {
	rootCmd.AddCommand(suggestCmd)
	suggestCmd.Flags().StringVarP(&suggestType, "type", "t", "general", "Type of suggestions (general, refactor, security, performance)")
	suggestCmd.Flags().IntVarP(&suggestLimit, "limit", "l", 5, "Maximum number of suggestions to generate")
	suggestCmd.Flags().StringSliceVarP(&suggestIgnore, "ignore", "i", nil, "Files or directories to ignore")
	suggestCmd.Flags().StringVarP(&suggestFocus, "focus", "f", ".", "Focus analysis on a specific path")
}

type Suggestion struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Type        string `json:"type"`
	File        string `json:"file,omitempty"`
}

func runSuggest(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Generate Context
	roots := []string{suggestFocus}
	if suggestFocus == "." {
		roots = []string{"."}
	} else {
		// Verify focus path exists
		if _, err := os.Stat(suggestFocus); err != nil {
			return fmt.Errorf("focus path does not exist: %w", err)
		}
	}

	opts := ContextOptions{
		Roots:     roots,
		Ignore:    suggestIgnore,
		MaxSize:   100 * 1024, // 100KB limit per file to save tokens
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing codebase...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-suggest")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`You are a senior software engineer conducting a code review.
Your goal is to identify impactful improvements, potential bugs, or technical debt.
Focus on: %s

Analyze the provided codebase context and list up to %d high-value suggestions.
Ignore trivial style issues.

Return the result as a raw JSON list of objects with the following structure:
[
  {
    "title": "Short title of the task",
    "description": "Detailed explanation of why this is needed and how to do it",
    "type": "refactor|bug|feature|security|perf",
    "file": "path/to/relevant/file (optional)"
  }
]

Do not wrap the JSON in markdown code blocks. Just return the raw JSON string.

CODEBASE CONTEXT:
%s`, suggestType, suggestLimit, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Consulting AI agent (this may take a moment)...")

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate suggestions: %w", err)
	}

	// 5. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var suggestions []Suggestion
	if err := json.Unmarshal([]byte(jsonStr), &suggestions); err != nil {
		// Fallback: try to print the raw response if parsing fails, maybe it's not JSON
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "Raw response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	if len(suggestions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No suggestions found. Your code looks great! (or try a different focus)")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nFound %d suggestions:\n\n", len(suggestions))

	// 6. Interactive Review
	for i, s := range suggestions {
		fmt.Fprintf(cmd.OutOrStdout(), "[%d/%d] %s (%s)\n", i+1, len(suggestions), s.Title, strings.ToUpper(s.Type))
		if s.File != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "      File: %s\n", s.File)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "      %s\n\n", s.Description)

		action := ""
		prompt := &survey.Select{
			Message: "What would you like to do?",
			Options: []string{"Add to TODO", "Skip", "Quit"},
			Default: "Add to TODO",
		}

		if err := askOneFunc(prompt, &action); err != nil {
			return err // Handle ctrl-c etc
		}

		if action == "Quit" {
			break
		}

		if action == "Add to TODO" {
			taskText := fmt.Sprintf("%s (%s)", s.Title, s.Type)
			if s.File != "" {
				// Rel path if possible
				if rel, err := filepath.Rel(cwd, s.File); err == nil {
					taskText += fmt.Sprintf(" - %s", rel)
				} else {
					taskText += fmt.Sprintf(" - %s", s.File)
				}
			}
			// Add description as a note? markdown doesn't support multiline list items easily in simple format.
			// Let's just stick to the title + context.

			if err := appendTask(taskText); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to add task: %v\n", err)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "---------------------------------------------------")
	}

	return nil
}
