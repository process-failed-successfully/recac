package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewAddTagsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-tags [file]",
		Short: "Add struct tags to Go code using AI",
		Long: `Reads a Go file or stdin and uses the configured AI agent to automatically
add struct tags (e.g., json, yaml, xml, db, bson) to all exported fields of structs.

Examples:
  recac add-tags my_structs.go --tags json,yaml
  recac add-tags my_structs.go --tags json,db --case snake
  cat models.go | recac add-tags --tags json --case camel`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			tagsFlag, _ := cmd.Flags().GetString("tags")
			caseFlag, _ := cmd.Flags().GetString("case")
			inPlace, _ := cmd.Flags().GetBool("in-place")
			showDiff, _ := cmd.Flags().GetBool("diff")

			if tagsFlag == "" {
				return errors.New("must provide at least one tag format via --tags (e.g., --tags json)")
			}

			var content []byte
			var filePath string
			var err error

			if len(args) > 0 {
				filePath = args[0]
				content, err = readFileFunc(filePath)
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

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-add-tags")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf(`Parse the following Go code and add the requested struct tags to all exported struct fields.
Requested tag formats: %s
Requested casing convention: %s (use this casing for the tag values based on the field name).

Do not change any logic, formatting (other than adding tags), or comments.
IMPORTANT: Return ONLY the raw modified Go code. Do not include any explanations, markdown formatting (like '''go ... '''), or conversational text.

Go Code:
'''
%s
'''`, tagsFlag, caseFlag, string(content))

			if filePath != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "Analyzing and adding tags to %s...\n", filePath)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Analyzing and adding tags to input...")
			}

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to add tags: %w", err)
			}

			updatedCode := utils.CleanCodeBlock(resp)
			updatedCode = strings.TrimSpace(updatedCode) + "\n"

			if showDiff {
				diff, err := utils.GenerateDiff(filePath, string(content), updatedCode)
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
				info, err := osStatFunc(filePath)
				if err != nil {
					return fmt.Errorf("failed to stat file: %w", err)
				}
				mode := info.Mode()

				if err := safeWriteFile(filePath, []byte(updatedCode), mode); err != nil {
					return fmt.Errorf("failed to write back to file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Successfully updated %s\n", filePath)
				return nil
			}

			// Default: print code to stdout
			fmt.Fprint(cmd.OutOrStdout(), updatedCode)

			return nil
		},
	}

	cmd.Flags().StringP("tags", "t", "json", "Comma-separated list of tag formats (e.g., json,yaml,db)")
	cmd.Flags().StringP("case", "c", "snake", "Casing convention for tag values (snake, camel, pascal, kebab)")
	cmd.Flags().Bool("diff", false, "Show diff between original and tagged code")
	cmd.Flags().BoolP("in-place", "i", false, "Modify the file in place (requires file argument)")

	return cmd
}

var addTagsCmd = NewAddTagsCmd()

func init() {
	rootCmd.AddCommand(addTagsCmd)
}
