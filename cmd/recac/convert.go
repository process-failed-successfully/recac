package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewConvertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "convert <input-file> <target-format> [output-file]",
		Short: "Convert a file from one format to another using AI",
		Long: `Converts the contents of an input file into a specified target format (e.g. JSON to YAML, CSV to JSON) using the configured AI agent.
If [output-file] is not provided, the result will be printed to stdout.`,
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			inputFile := args[0]
			targetFormat := args[1]
			var outputFile string
			if len(args) == 3 {
				outputFile = args[2]
			}

			// Read input file
			content, err := readFileFunc(inputFile)
			if err != nil {
				return fmt.Errorf("failed to read input file: %w", err)
			}

			if len(content) == 0 {
				return fmt.Errorf("input file is empty")
			}

			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, _ := os.Getwd()

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-convert")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf(`Convert the following content to %s format.
Output ONLY the converted content without any conversational text.
If the content cannot be reasonably converted to %s, return an error message explaining why.

Content:
%s`, targetFormat, targetFormat, string(content))

			if outputFile != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Converting %s to %s...\n", inputFile, targetFormat)
			}

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to convert file: %w", err)
			}

			// Clean any markdown code blocks that the AI might have added
			resp = utils.CleanCodeBlock(resp)

			if outputFile != "" {
				err = writeFileFunc(outputFile, []byte(resp), 0644)
				if err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Successfully converted and saved to %s\n", outputFile)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), resp)
			}

			return nil
		},
	}
}

var convertCmd = NewConvertCmd()

func init() {
	rootCmd.AddCommand(convertCmd)
}
