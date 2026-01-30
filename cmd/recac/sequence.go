package main

import (
	"fmt"
	"os"
	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	sequenceOutput string
	sequenceFocus  []string
	sequenceIgnore []string
)

var sequenceCmd = &cobra.Command{
	Use:   "sequence \"Scenario description\"",
	Short: "Generate a Mermaid sequence diagram for a scenario",
	Long: `Analyze the codebase and generate a Mermaid sequence diagram for a specific user-defined scenario.
This uses the configured AI agent to trace the potential execution flow based on the static code.`,
	Args: cobra.ExactArgs(1),
	RunE: runSequence,
}

func init() {
	rootCmd.AddCommand(sequenceCmd)
	sequenceCmd.Flags().StringVarP(&sequenceOutput, "output", "o", "", "Output file path (e.g. sequence.mmd)")
	sequenceCmd.Flags().StringSliceVarP(&sequenceFocus, "focus", "f", []string{"."}, "Files or directories to focus on (defaults to current directory)")
	sequenceCmd.Flags().StringSliceVarP(&sequenceIgnore, "ignore", "i", nil, "Additional ignore patterns")
}

func runSequence(cmd *cobra.Command, args []string) error {
	scenario := args[0]
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Generate Context
	// We use the focus list as roots
	var roots []string
	for _, f := range sequenceFocus {
		// Resolve relative to cwd if needed, or just pass as is.
		roots = append(roots, f)
	}

	opts := ContextOptions{
		Roots:     roots,
		Ignore:    sequenceIgnore,
		MaxSize:   100 * 1024, // 100KB per file limit to allow more files
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing codebase context...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-sequence")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are an expert software architect.
Your task is to generate a Mermaid Sequence Diagram based on the provided Code Context for the following Scenario.

Scenario: "%s"

Instructions:
1. Analyze the code to understand the flow of control for this scenario.
2. Identify the key participants (classes, functions, services, files).
3. Generate a valid Mermaid Sequence Diagram (starting with 'sequenceDiagram').
4. Use 'participant' or 'actor' to define entities.
5. Focus on the high-level flow and critical interactions.
6. Return ONLY the mermaid code block. Do not include explanations.

Code Context:
%s
`, scenario, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating sequence diagram...")

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Process Output
	mermaidCode := utils.CleanCodeBlock(resp)
	if mermaidCode == "" {
		// Fallback: use raw response if it looks like mermaid but wasn't in a block
		if len(resp) >= 15 && resp[0:15] == "sequenceDiagram" {
			mermaidCode = resp
		} else if len(resp) > 0 {
			// If it's not starting with sequenceDiagram but we have content, maybe it's just missing the tag?
			// Or maybe the agent failed.
			// Let's warn but output it.
			fmt.Fprintln(cmd.ErrOrStderr(), "Warning: Agent response does not look like a standard mermaid block.")
			mermaidCode = resp
		} else {
			return fmt.Errorf("agent returned empty response")
		}
	}

	if sequenceOutput != "" {
		if err := os.WriteFile(sequenceOutput, []byte(mermaidCode), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Sequence diagram saved to %s\n", sequenceOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), mermaidCode)
	}

	return nil
}
