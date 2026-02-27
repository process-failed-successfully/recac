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
	translateTarget      string
	translateOutput      string
	translateInstruction string
)

var translateCmd = &cobra.Command{
	Use:   "translate [file]",
	Short: "Translate code to another language",
	Long: `Translate source code from one language to another using AI.
This command preserves logic and comments while adapting to the target language's idioms.

Example:
  recac translate script.py --target go
  recac translate Legacy.java --target kotlin --output New.kt
`,
	Args: cobra.ExactArgs(1),
	RunE: runTranslate,
}

func init() {
	rootCmd.AddCommand(translateCmd)
	translateCmd.Flags().StringVarP(&translateTarget, "target", "t", "", "Target language (e.g., go, python, rust) (required)")
	translateCmd.Flags().StringVarP(&translateOutput, "output", "o", "", "Output file path (default: stdout)")
	translateCmd.Flags().StringVarP(&translateInstruction, "instruction", "i", "", "Additional instructions (e.g., 'use async/await')")
	translateCmd.MarkFlagRequired("target")
}

func runTranslate(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// 1. Read input file
	content, err := readFileFunc(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	if len(content) == 0 {
		return fmt.Errorf("input file is empty")
	}

	// 2. Prepare Agent
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-translate")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// 3. Construct Prompt
	prompt := fmt.Sprintf(`You are an expert polyglot programmer.
Translate the following code to %s.

Requirements:
1. Preserve the original logic and comments.
2. Use idiomatic patterns for %s.
3. Return ONLY the translated code. Do not include markdown formatting (like '''code...''') or explanations.
`, translateTarget, translateTarget)

	if translateInstruction != "" {
		prompt += fmt.Sprintf("4. Additional instruction: %s\n", translateInstruction)
	}

	prompt += fmt.Sprintf("\nInput Code (%s):\n'''\n%s\n'''", filePath, string(content))

	fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Translating %s to %s...\n", filePath, translateTarget)

	// 4. Send to Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 5. Clean Output
	translatedCode := utils.CleanCodeBlock(resp)

	// 6. Output
	if translateOutput != "" {
		if err := writeFileFunc(translateOutput, []byte(translatedCode), 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Translated code saved to %s\n", translateOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), translatedCode)
	}

	return nil
}
