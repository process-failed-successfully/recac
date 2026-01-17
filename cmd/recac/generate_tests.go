package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewGenerateTestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate-tests [file]",
		Aliases: []string{"gen-tests", "test-gen"},
		Short:   "Generate unit tests for a file",
		Long:    `Reads a file and uses the configured AI agent to generate unit tests for it.`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var content []byte
			var fileName string
			var err error

			if len(args) > 0 {
				fileName = args[0]
				content, err = os.ReadFile(fileName)
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
			} else {
				// Read from stdin
				in := cmd.InOrStdin()
				content, err = io.ReadAll(in)
				if err != nil {
					return fmt.Errorf("failed to read from input: %w", err)
				}
				fileName = "stdin"
			}

			if len(content) == 0 {
				return errors.New("input is empty")
			}

			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			framework, _ := cmd.Flags().GetString("framework")

			cwd, _ := os.Getwd()

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-gen-tests")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf("Please generate unit tests for the following code (file: %s).\n", fileName)
			if framework != "" {
				prompt += fmt.Sprintf("Use the '%s' testing framework.\n", framework)
			} else {
				prompt += "Infer the best testing framework for the language.\n"
			}
			prompt += "Return the test code. If possible, enclose it in a markdown code block.\n\n"
			prompt += fmt.Sprintf("```\n%s\n```", string(content))

			fmt.Fprintf(cmd.ErrOrStderr(), "Generating tests for %s...\n", fileName)

			// Determine if we need to buffer for file output
			outputFile, _ := cmd.Flags().GetString("output")
			var outputBuffer *strings.Builder
			if outputFile != "" {
				outputBuffer = &strings.Builder{}
			}

			_, err = ag.SendStream(ctx, prompt, func(chunk string) {
				if outputFile != "" {
					outputBuffer.WriteString(chunk)
				} else {
					fmt.Fprint(cmd.OutOrStdout(), chunk)
				}
			})
			if err != nil {
				return err
			}

			if outputFile != "" {
				// Try to extract code block
				fullResponse := outputBuffer.String()
				code := extractCodeBlock(fullResponse)

				// Write to file
				if err := os.WriteFile(outputFile, []byte(code), 0644); err != nil {
					return fmt.Errorf("failed to write output file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "\nTests written to %s\n", outputFile)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "")
			}

			return nil
		},
	}

	cmd.Flags().StringP("framework", "f", "", "Testing framework to use (e.g., testing, pytest, jest)")
	cmd.Flags().StringP("output", "o", "", "Write output to file")

	return cmd
}

func extractCodeBlock(response string) string {
	// Simple extractor for ``` code blocks
	start := strings.Index(response, "```")
	if start == -1 {
		return response
	}

	// Skip the opening ``` and optional language identifier
	rest := response[start+3:]
	newline := strings.Index(rest, "\n")
	if newline != -1 {
		rest = rest[newline+1:]
	}

	end := strings.LastIndex(rest, "```")
	if end == -1 {
		return rest // No closing block, return everything from start
	}

	return rest[:end]
}

var generateTestsCmd = NewGenerateTestsCmd()

func init() {
	rootCmd.AddCommand(generateTestsCmd)
}
