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

func NewTypeGenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "typegen [file]",
		Short: "Generate structs/types from JSON or SQL payloads",
		Long: `Reads a JSON or SQL file (or stdin) and uses the AI agent to generate
strongly-typed data structures (e.g., structs, interfaces, dataclasses) in the target language.

Examples:
  recac typegen payload.json --lang go --name UserPayload
  cat schema.sql | recac typegen --lang typescript`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			lang, _ := cmd.Flags().GetString("lang")
			name, _ := cmd.Flags().GetString("name")

			var content []byte
			var err error

			if len(args) > 0 {
				filePath := args[0]
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

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-typegen")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf(`Convert the following payload (JSON or SQL) into %s data structures (like structs, interfaces, or classes).
Use the root name "%s" for the top-level structure.
Ensure proper typing, tags (e.g., json omitempty), and formatting conventions for %s.
Return ONLY the code block, with NO conversational text.

Payload:
%s
`, lang, name, lang, string(content))

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed: %w", err)
			}

			// Clean up output to only contain the code block if it was wrapped in markdown
			cleanResp := utils.CleanCodeBlock(resp)

			fmt.Fprintln(cmd.OutOrStdout(), cleanResp)

			return nil
		},
	}

	cmd.Flags().String("lang", "go", "Target programming language (e.g., go, typescript, python)")
	cmd.Flags().String("name", "Root", "Name of the top-level struct/type")

	return cmd
}

func init() {
	rootCmd.AddCommand(NewTypeGenCmd())
}
