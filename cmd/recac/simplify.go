package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	simplifyInPlace bool
)

var simplifyCmd = &cobra.Command{
	Use:   "simplify [file]",
	Short: "Simplify complex code using AI to improve readability and maintainability",
	Long: `Reads a source file and asks the configured AI agent to simplify the code
(e.g. reduce cyclomatic complexity, improve readability, extract methods)
without changing its behavior.

Examples:
  recac simplify main.go
  recac simplify main.go --in-place
`,
	Args: cobra.ExactArgs(1),
	RunE: runSimplify,
}

func init() {
	rootCmd.AddCommand(simplifyCmd)
	simplifyCmd.Flags().BoolVarP(&simplifyInPlace, "in-place", "i", false, "Overwrite the file in place with the simplified code")
}

func runSimplify(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	stat, err := osStatFunc(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	content, err := readFileFunc(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-simplify")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert Senior Software Engineer.
Your task is to simplify the following code to improve its readability and maintainability,
and to reduce its cyclomatic complexity.
Do NOT change the functional behavior of the code.
Return ONLY the refactored code block. Do not include explanations.

File: %s
Code:
%s
`, filePath, string(content))

	fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Analyzing and simplifying %s...\n", filePath)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to simplify code: %w", err)
	}

	simplifiedCode := utils.CleanCodeBlock(resp)

	if simplifyInPlace {
		if err := writeFileFunc(filePath, []byte(simplifiedCode), stat.Mode()); err != nil {
			return fmt.Errorf("failed to write simplified code to %s: %w", filePath, err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Successfully simplified and updated %s in place.\n", filePath)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), simplifiedCode)
	}

	return nil
}
