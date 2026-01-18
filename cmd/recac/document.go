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

func NewDocumentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "document [file]",
		Short: "Add documentation comments to code using AI",
		Long:  `Reads a file or stdin and asks the configured AI agent to add documentation comments (e.g., GoDoc, Javadoc, Docstrings) to the code.`,
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

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-document")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			style, _ := cmd.Flags().GetString("style")
			if style == "" {
				style = "appropriate for the language"
			}

			// Construct the prompt
			prompt := fmt.Sprintf(`You are an expert technical writer.
Add comprehensive documentation comments to the following code.
Use the style: %s.
Do not change the code logic, indentation, or structure. ONLY add comments.
Return ONLY the documented code. Do not include any explanations or markdown formatting outside of the code block.

Code to document:
'''
%s
'''`, style, string(content))

			fmt.Fprintln(cmd.ErrOrStderr(), "Generating documentation...")

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to document code: %w", err)
			}

			if !strings.Contains(resp, "```") {
				fmt.Fprintln(cmd.ErrOrStderr(), "Warning: No markdown code block found in agent response. Using raw output.")
			}

			documentedCode := cleanCodeFromMarkdown(resp)

			inPlace, _ := cmd.Flags().GetBool("in-place")

			if inPlace {
				if filePath == "" {
					return errors.New("cannot use --in-place with stdin input")
				}
				if err := os.WriteFile(filePath, []byte(documentedCode), 0644); err != nil {
					return fmt.Errorf("failed to write back to file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Successfully updated %s\n", filePath)
				return nil
			}

			// Default: print documented code to stdout
			fmt.Fprint(cmd.OutOrStdout(), documentedCode)

			return nil
		},
	}

	cmd.Flags().BoolP("in-place", "i", false, "Modify the file in place (requires file argument)")
	cmd.Flags().StringP("style", "s", "", "Documentation style (e.g., godoc, javadoc, python-docstring)")

	return cmd
}

var documentCmd = NewDocumentCmd()

func init() {
	rootCmd.AddCommand(documentCmd)
}

// cleanCodeFromMarkdown strips markdown code blocks if present
func cleanCodeFromMarkdown(content string) string {
	content = strings.TrimSpace(content)

	// Try to find markdown code blocks
	start := strings.Index(content, "```")
	if start != -1 {
		// Found a code block start
		// Skip the opening ``` and potential language identifier
		codeStart := start + 3

		// Find the end of the line to skip language identifier (e.g., ```go)
		if idx := strings.Index(content[codeStart:], "\n"); idx != -1 {
			codeStart += idx + 1
		}

		// Find the end of the block
		end := strings.Index(content[codeStart:], "```")
		if end != -1 {
			// Extract the content inside the block
			return strings.TrimSpace(content[codeStart : codeStart+end])
		}
	}

	return content
}
