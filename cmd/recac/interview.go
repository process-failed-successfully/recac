package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	interviewOutput string
)

var interviewCmd = &cobra.Command{
	Use:   "interview [topic]",
	Short: "Interactively gather requirements and generate a spec",
	Long: `Starts an interactive interview session with an AI Requirements Engineer.
The agent will ask clarifying questions about your project idea and, once satisfied,
generate a comprehensive specification file (default: app_spec.txt).`,
	RunE: runInterview,
}

func init() {
	rootCmd.AddCommand(interviewCmd)
	interviewCmd.Flags().StringVarP(&interviewOutput, "output", "o", "app_spec.txt", "Output specification file")
}

func runInterview(cmd *cobra.Command, args []string) error {
	// 1. Check Output File
	if _, err := os.Stat(interviewOutput); err == nil {
		overwrite := false
		err := askOneFunc(&survey.Confirm{
			Message: fmt.Sprintf("File '%s' already exists. Overwrite?", interviewOutput),
			Default: false,
		}, &overwrite)
		if err != nil {
			return err // User cancelled
		}
		if !overwrite {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// 2. Initialize Agent
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory for testability
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-interview")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Construct System Prompt
	systemPrompt := `You are an expert Requirements Engineer. Your goal is to create a detailed software specification for the user's idea.
Follow this process:
1. Ask ONE clarifying question at a time to gather necessary details (core features, technology preferences, target audience, constraints).
2. Keep your questions concise.
3. When you have gathered sufficient information to build a solid v1 spec, output the final specification wrapped in a code block like this:
'''spec
[Title]
[Overview]
[Features]
...
'''
4. IMMEDIATELLY after the spec block, output the exact text "SPEC_COMPLETE" on a new line.

Do not generate the spec until you have enough detail. Start by asking about the core idea if not provided.`

	// 4. Initial User Input
	var userInput string
	if len(args) > 0 {
		userInput = strings.Join(args, " ")
	} else {
		err := askOneFunc(&survey.Input{
			Message: "What would you like to build today?",
		}, &userInput)
		if err != nil {
			return nil // User cancelled
		}
	}

	// 5. Conversation Loop
	history := fmt.Sprintf("%s\n\nUser: %s\n", systemPrompt, userInput)

	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Interviewer Agent:")

	for {
		// Send history to agent
		resp, err := ag.Send(ctx, history)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), resp)
		fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

		// Update history with Agent response
		history += fmt.Sprintf("\nAgent: %s\n", resp)

		// Check for completion
		if strings.Contains(resp, "SPEC_COMPLETE") {
			// Extract Spec
			spec := extractSpec(resp)
			if spec == "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "Warning: Agent said SPEC_COMPLETE but no spec block found.")
				// Fallback: save whole response?
				spec = resp
			}

			if err := os.WriteFile(interviewOutput, []byte(spec), 0644); err != nil {
				return fmt.Errorf("failed to write spec file: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n✅ Specification generated and saved to %s\n", interviewOutput)
			break
		}

		// Ask User for next input
		var nextInput string
		err = askOneFunc(&survey.Input{
			Message: "Your answer:",
		}, &nextInput)
		if err != nil {
			return nil // User cancelled (Ctrl+C)
		}

		// Update history with User response
		history += fmt.Sprintf("\nUser: %s\n", nextInput)
	}

	return nil
}

func extractSpec(response string) string {
	// Look for '''spec or ```spec
	markers := []string{"```spec", "'''spec"}

	for _, marker := range markers {
		start := strings.Index(response, marker)
		if start != -1 {
			rest := response[start+len(marker):]
			// Find end marker
			endMarker := "```"
			if strings.HasPrefix(marker, "'''") {
				endMarker = "'''"
			}

			end := strings.Index(rest, endMarker)
			if end != -1 {
				return strings.TrimSpace(rest[:end])
			}
		}
	}

	return ""
}
