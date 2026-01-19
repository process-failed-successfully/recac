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

func NewImproveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "improve [file]",
		Short: "Improve code using AI",
		Long:  `Reads a file or stdin and asks the configured AI agent to improve the code (refactor, fix bugs, etc.).`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var content []byte
			var filePath string
			var err error

			if len(args) > 0 {
				filePath = args[0]
				content, err = os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
			} else {
				// Stdin logic
				in := cmd.InOrStdin()
				if f, ok := in.(*os.File); ok && f == os.Stdin {
					stat, _ := f.Stat()
					if (stat.Mode() & os.ModeCharDevice) != 0 {
						return errors.New("please provide a file path or pipe content via stdin")
					}
				}

				content, err = io.ReadAll(in)
				if err != nil {
					return fmt.Errorf("failed to read from input: %w", err)
				}
			}

			if len(content) == 0 {
				return errors.New("input is empty")
			}

			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-improve")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			userPrompt, _ := cmd.Flags().GetString("prompt")
			if userPrompt == "" {
				userPrompt = "Improve this code by fixing bugs, optimizing performance, and ensuring best practices."
			}

			// Construct the full prompt
			prompt := fmt.Sprintf(`%s

IMPORTANT: Return ONLY the improved code. Do not include any explanations, markdown formatting (like '''go ... '''), or conversational text. just the raw code.

Code to improve:
'''
%s
'''`, userPrompt, string(content))

			fmt.Fprintln(cmd.ErrOrStderr(), "Analyzing and improving code...")

			// We need the full response to process it
			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to improve code: %w", err)
			}

			improvedCode := utils.CleanCodeBlock(resp)

			showDiff, _ := cmd.Flags().GetBool("diff")
			inPlace, _ := cmd.Flags().GetBool("in-place")

			if showDiff {
				diff, err := utils.GenerateDiff(filePath, string(content), improvedCode)
				if err != nil {
					return fmt.Errorf("failed to generate diff: %w", err)
				}
				fmt.Fprint(cmd.OutOrStdout(), diff)
				return nil
			}

			if inPlace {
				if filePath == "" {
					return errors.New("cannot use --in-place with stdin input")
				}
				if err := os.WriteFile(filePath, []byte(improvedCode), 0644); err != nil {
					return fmt.Errorf("failed to write back to file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Successfully updated %s\n", filePath)
				return nil
			}

			// Default: print improved code to stdout
			fmt.Fprint(cmd.OutOrStdout(), improvedCode)

			return nil
		},
	}

	cmd.Flags().Bool("diff", false, "Show diff between original and improved code")
	cmd.Flags().BoolP("in-place", "i", false, "Modify the file in place (requires file argument)")
	cmd.Flags().StringP("prompt", "p", "", "Custom instruction for improvement")

	return cmd
}

var improveCmd = NewImproveCmd()

func init() {
	rootCmd.AddCommand(improveCmd)
}
