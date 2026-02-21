package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	refineOutput string
	refineModel  string
)

var refineCmd = &cobra.Command{
	Use:   "refine [initial prompt]",
	Short: "Interactively refine a project specification",
	Long: `Starts an interactive session with the AI agent to clarify requirements and generate a detailed specification (app_spec.txt).
The agent will ask you questions until it has enough information to create a comprehensive spec.`,
	RunE: runRefine,
}

func init() {
	rootCmd.AddCommand(refineCmd)
	refineCmd.Flags().StringVarP(&refineOutput, "output", "o", "app_spec.txt", "Output file for the specification")
	// Allow overriding model specifically for refinement (might need a smarter model)
	refineCmd.Flags().StringVar(&refineModel, "model", "", "Override AI model for refinement")
}

func runRefine(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// 1. Initial State
	var conversationHistory strings.Builder
	initialPrompt := ""
	if len(args) > 0 {
		initialPrompt = strings.Join(args, " ")
		conversationHistory.WriteString(fmt.Sprintf("User: %s\n", initialPrompt))
	} else {
		fmt.Fprint(cmd.OutOrStdout(), "Describe what you want to build: ")
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if scanner.Scan() {
			initialPrompt = scanner.Text()
			conversationHistory.WriteString(fmt.Sprintf("User: %s\n", initialPrompt))
		} else {
			return fmt.Errorf("no input provided")
		}
	}

	// 2. Setup Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	if refineModel != "" {
		model = refineModel
	}

	projectName := filepath.Base(cwd)
	ag, err := agentClientFactory(ctx, provider, model, cwd, projectName)
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n🧠 Thinking... (Press Ctrl+C to exit)")

	// 3. Interactive Loop
	// We limit turns to prevent infinite loops, though user can stop anytime.
	maxTurns := 20

	for i := 0; i < maxTurns; i++ {
		// Construct the system prompt for the current turn
		prompt := fmt.Sprintf(`You are an expert Product Manager and Systems Architect.
Your goal is to create a detailed technical specification (app_spec.txt) based on the user's requirements.

Conversation History:
%s

Task:
Analyze the conversation so far.
1. If the requirements are vague or missing critical details (e.g., tech stack, database, key features, scale), ask ONE clarifying question.
2. If the requirements are clear enough to build a solid v1, output the final specification.

Format your response EXACTLY as follows:

If asking a question:
QUESTION: <your question here>

If the spec is ready:
DONE
SPEC:
<content of app_spec.txt>

Do not include any other text before or after.
`, conversationHistory.String())

		// Send to agent
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		resp = strings.TrimSpace(resp)

		// Check for DONE
		if strings.HasPrefix(resp, "DONE") {
			// Extract spec
			specContent := ""
			if parts := strings.SplitN(resp, "SPEC:", 2); len(parts) > 1 {
				specContent = strings.TrimSpace(parts[1])
			} else {
				// Fallback if SPEC: tag is missing but DONE is present (rare)
				specContent = strings.TrimPrefix(resp, "DONE")
			}

			// Clean up markdown code blocks if present
			if strings.HasPrefix(specContent, "```") {
				lines := strings.Split(specContent, "\n")
				if len(lines) >= 2 {
					// Remove first and last line
					specContent = strings.Join(lines[1:len(lines)-1], "\n")
				}
			}

			// Write to file
			if err := os.WriteFile(refineOutput, []byte(specContent), 0644); err != nil {
				return fmt.Errorf("failed to write spec file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\n✅ Specification generated and saved to %s\n", refineOutput)
			return nil
		}

		// Check for QUESTION
		if strings.HasPrefix(resp, "QUESTION:") {
			question := strings.TrimSpace(strings.TrimPrefix(resp, "QUESTION:"))
			fmt.Fprintf(cmd.OutOrStdout(), "\n🤖 %s\n", question)

			// Record Agent's question
			conversationHistory.WriteString(fmt.Sprintf("Agent: %s\n", question))

			// Get User Answer
			fmt.Fprint(cmd.OutOrStdout(), "> ")
			scanner := bufio.NewScanner(cmd.InOrStdin())
			if scanner.Scan() {
				answer := scanner.Text()
				conversationHistory.WriteString(fmt.Sprintf("User: %s\n", answer))
			} else {
				// EOF or error
				return fmt.Errorf("input stream closed")
			}
			continue
		}

		// Fallback: Agent messed up format. Treat whole response as a question?
		// Or try to parse it. Let's treat it as a question if it ends with '?'
		if strings.HasSuffix(resp, "?") {
			fmt.Fprintf(cmd.OutOrStdout(), "\n🤖 %s\n", resp)
			conversationHistory.WriteString(fmt.Sprintf("Agent: %s\n", resp))

			fmt.Fprint(cmd.OutOrStdout(), "> ")
			scanner := bufio.NewScanner(cmd.InOrStdin())
			if scanner.Scan() {
				answer := scanner.Text()
				conversationHistory.WriteString(fmt.Sprintf("User: %s\n", answer))
			}
			continue
		}

		// Agent failed format completely
		fmt.Fprintf(cmd.ErrOrStderr(), "\n⚠️  Agent returned unexpected format:\n%s\n", resp)
		// We can try to continue or just fail. Let's ask user to rephrase.
		fmt.Fprint(cmd.OutOrStdout(), "> (Agent was confused, please clarify) ")
		scanner := bufio.NewScanner(cmd.InOrStdin())
		if scanner.Scan() {
			answer := scanner.Text()
			conversationHistory.WriteString(fmt.Sprintf("User: %s\n", answer))
		}
	}

	return fmt.Errorf("refinement session exceeded maximum turns (%d)", maxTurns)
}
