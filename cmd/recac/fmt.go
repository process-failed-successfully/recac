package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fmtCmd = &cobra.Command{
	Use:   "fmt [files...]",
	Short: "Format and fix stylistic issues in code using AI",
	Long: `Reads the specified source files and asks the configured AI agent to act as a
code formatter/linter. It will fix formatting, styling, and stylistic issues in the files
and overwrite them with the formatted code.

Examples:
  recac fmt main.go utils.go
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runFmt,
}

func init() {
	rootCmd.AddCommand(fmtCmd)
}

func runFmt(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-fmt")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	for _, filePath := range args {
		stat, err := osStatFunc(filePath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to stat file %s: %v\n", filePath, err)
			continue
		}

		if stat.IsDir() {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: skipping directory %s\n", filePath)
			continue
		}

		content, err := readFileFunc(filePath)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read file %s: %v\n", filePath, err)
			continue
		}

		prompt := fmt.Sprintf(`You are an expert code formatter and linter.
Your task is to fix any code formatting, styling, or stylistic issues in the provided file.
Do NOT change the functional behavior of the code.

Return the FULL CONTENT of the modified file wrapped in <file path="%s">...</file> tags.
Do not include explanations or diffs.

<file path="%s">
%s
</file>
`, filePath, filePath, string(content))

		fmt.Fprintf(cmd.ErrOrStderr(), "🤖 Formatting %s...\n", filePath)

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: agent failed to format %s: %v\n", filePath, err)
			continue
		}

		files := utils.ParseFileBlocks(resp)

		if len(files) == 0 {
			// fallback: try CleanCodeBlock if the model didn't use the tag properly
			formattedCode := utils.CleanCodeBlock(resp)
			if formattedCode != "" {
				files = map[string]string{filePath: formattedCode}
			}
		}

		formattedContent, ok := files[filePath]
		if !ok {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: agent did not return formatted code for %s in the expected format.\n", filePath)
			continue
		}

		if err := writeFileFunc(filePath, []byte(formattedContent), stat.Mode()); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to write formatted code to %s: %v\n", filePath, err)
			continue
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "✅ Successfully formatted %s\n", filePath)
	}

	return nil
}
