package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	portFrom        string
	portTo          string
	portOutput      string
	portInstruction string
)

var portCmd = &cobra.Command{
	Use:   "port [file]",
	Short: "Port code from one language to another using AI",
	Long: `Translate a source code file to another language or framework using the configured AI agent.
This command is useful for migrations, rewrites, or learning how code translates between languages.

Example:
  recac port src/utils.py --to go --output src/utils.go
  recac port component.jsx --to vue --instruction "Use Composition API"`,
	Args: cobra.ExactArgs(1),
	RunE: runPort,
}

func init() {
	rootCmd.AddCommand(portCmd)
	portCmd.Flags().StringVar(&portFrom, "from", "", "Source language (optional, auto-detected from extension)")
	portCmd.Flags().StringVar(&portTo, "to", "", "Target language (required)")
	portCmd.Flags().StringVarP(&portOutput, "output", "o", "", "Output file path (default: stdout)")
	portCmd.Flags().StringVar(&portInstruction, "instruction", "", "Additional instructions for the porting process")

	portCmd.MarkFlagRequired("to")
}

func runPort(cmd *cobra.Command, args []string) error {
	inputFile := args[0]

	// 1. Read Input
	content, err := os.ReadFile(inputFile)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// 2. Prepare Context
	ctx := cmd.Context() // Use command context (important for tests)
	if ctx == nil {
		ctx = context.Background()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// 3. Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-port")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 4. Construct Prompt
	var sb strings.Builder
	sb.WriteString("You are an expert Polyglot Programmer.\n")
	sb.WriteString(fmt.Sprintf("Port the following code to %s.\n", portTo))

	if portFrom != "" {
		sb.WriteString(fmt.Sprintf("Source Language: %s\n", portFrom))
	}
	if portInstruction != "" {
		sb.WriteString(fmt.Sprintf("Instructions: %s\n", portInstruction))
	}

	sb.WriteString("\nRequirements:\n")
	sb.WriteString("- The code must be idiomatic in the target language.\n")
	sb.WriteString("- Preserve the logic and functionality of the original code.\n")
	sb.WriteString("- Return ONLY the ported code. Do not include markdown code blocks or explanations.\n")

	sb.WriteString("\nCode:\n")
	sb.WriteString("'''\n")
	sb.WriteString(string(content))
	sb.WriteString("\n'''\n")

	fmt.Fprintf(cmd.ErrOrStderr(), "🚀 Porting %s to %s...\n", inputFile, portTo)

	// 5. Send to Agent
	resp, err := ag.Send(ctx, sb.String())
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 6. Clean Output
	cleaned := utils.CleanCodeBlock(resp)

	// 7. Write Output
	if portOutput != "" {
		if err := os.WriteFile(portOutput, []byte(cleaned), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Ported code written to %s\n", portOutput)
	} else {
		// Write to stdout
		// We explicitly use cmd.OutOrStdout() for testability
		io.WriteString(cmd.OutOrStdout(), cleaned)
	}

	return nil
}
