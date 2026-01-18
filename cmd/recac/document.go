package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

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

			// Construct the prompt
			prompt := fmt.Sprintf(`Add documentation comments to the following code.
Use the standard convention for the language (e.g., GoDoc for Go, Docstrings for Python, Javadoc for Java).
Document exported functions, types, constants, and complex logic.
Do not change the logic or existing code, only add comments.
IMPORTANT: Return ONLY the code with documentation. Do not include any explanations, markdown formatting (like '''go ... '''), or conversational text.

Code to document:
'''
%s
'''`, string(content))

			fmt.Fprintln(cmd.ErrOrStderr(), "Analyzing and documenting code...")

			// We need the full response to process it
			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to document code: %w", err)
			}

			documentedCode := cleanCode(resp)

			showDiff, _ := cmd.Flags().GetBool("diff")
			inPlace, _ := cmd.Flags().GetBool("in-place")

			if showDiff {
				diff, err := generateDiff(filePath, string(content), documentedCode)
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

				// Preserve file permissions
				info, err := os.Stat(filePath)
				if err != nil {
					return fmt.Errorf("failed to stat file: %w", err)
				}
				mode := info.Mode()

				if err := os.WriteFile(filePath, []byte(documentedCode), mode); err != nil {
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

	cmd.Flags().Bool("diff", false, "Show diff between original and documented code")
	cmd.Flags().BoolP("in-place", "i", false, "Modify the file in place (requires file argument)")

	return cmd
}

var documentCmd = NewDocumentCmd()

func init() {
	rootCmd.AddCommand(documentCmd)
}

// Note: cleanCode and generateDiff are reused from improve.go if they are exported or if I duplicate them.
// Since I can't guarantee they are exported or accessible (they are in the same package 'main', so if they are top-level they are accessible),
// I will check if `improve.go` defines them as top-level functions in package main.
// Based on previous read of `improve.go`, `cleanCode` and `generateDiff` ARE top-level functions in package main.
// So I don't need to redefine them here if `improve.go` is part of the build.
// However, to be safe and avoid compilation errors if `improve.go` changes, I might rename them or duplicate them if I'm not sure.
// Wait, in Go, all files in the same package see each other's unexported symbols.
// `cleanCode` in `improve.go` starts with lowercase, so it's unexported from the package, but visible within `package main`.
// So I can use them directly!
// BUT, if I redefine them, I'll get a "redeclared" error.
// So I should NOT redefine them if they exist.
// Let me verify `improve.go` content again to be 100% sure they are top level.
