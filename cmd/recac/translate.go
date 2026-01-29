package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	translateFrom string
	translateTo   string
	translateOut  string
	translateCode string
)

func NewTranslateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "translate [file]",
		Short: "Translate code from one language to another using AI",
		Long:  `Translates a file or code snippet into another programming language.`,
		Example: `  recac translate main.py --to go
  recac translate --code "console.log('hello')" --to python
  recac translate script.rb --to js --out script.js`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var content string
			var err error

			// 1. Determine Input
			if translateCode != "" {
				content = translateCode
			} else if len(args) > 0 {
				filePath := args[0]
				fileBytes, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", filePath, err)
				}
				content = string(fileBytes)
			} else {
				// Try Stdin
				in := cmd.InOrStdin()
				// Check if it's interactive terminal (empty stdin)
				if f, ok := in.(*os.File); ok && f == os.Stdin {
					stat, _ := f.Stat()
					if (stat.Mode() & os.ModeCharDevice) != 0 {
						return errors.New("please provide a file, --code, or pipe content via stdin")
					}
				}

				bytes, err := io.ReadAll(in)
				if err != nil {
					return fmt.Errorf("failed to read from input: %w", err)
				}
				content = string(bytes)
			}

			if content == "" {
				return errors.New("input content is empty")
			}

			// 2. Validate Flags
			if translateTo == "" {
				return errors.New("target language is required (--to)")
			}

			// 3. Prepare Agent
			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, _ := os.Getwd()

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-translate")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			// 4. Construct Prompt
			fromClause := ""
			if translateFrom != "" {
				fromClause = fmt.Sprintf(" from %s", translateFrom)
			}

			prompt := fmt.Sprintf(`Translate the following code%s to %s.
Return ONLY the translated code. Do not include markdown formatting (like '''go ... ''') or explanations.

Code:
'''
%s
'''`, fromClause, translateTo, content)

			fmt.Fprintf(cmd.ErrOrStderr(), "Translating%s to %s...\n", fromClause, translateTo)

			// 5. Call Agent
			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent translation failed: %w", err)
			}

			translatedCode := utils.CleanCodeBlock(resp)

			// 6. Output
			if translateOut != "" {
				if err := os.WriteFile(translateOut, []byte(translatedCode), 0644); err != nil {
					return fmt.Errorf("failed to write output to %s: %w", translateOut, err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Translation saved to %s\n", translateOut)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), translatedCode)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&translateFrom, "from", "", "Source language (optional, auto-detected if omitted)")
	cmd.Flags().StringVar(&translateTo, "to", "", "Target language (required)")
	cmd.Flags().StringVar(&translateOut, "out", "", "Output file path (optional, prints to stdout if omitted)")
	cmd.Flags().StringVar(&translateCode, "code", "", "Inline code snippet to translate")

	return cmd
}

var translateCmd = NewTranslateCmd()

func init() {
	rootCmd.AddCommand(translateCmd)
}
