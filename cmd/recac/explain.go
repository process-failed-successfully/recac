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

func NewExplainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "explain [file]",
		Short: "Explain code using AI",
		Long:  `Reads a file or stdin and asks the configured AI agent to explain the code.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var content []byte
			var err error

			if len(args) > 0 {
				// Read from file
				content, err = os.ReadFile(args[0])
				if err != nil {
					return fmt.Errorf("failed to read file: %w", err)
				}
			} else {
				// Check if input is available from stdin
				// When running in tests with SetIn, checking os.Stdin.Stat is not reliable/correct for the buffer.
				// We rely on cmd.InOrStdin()

				// However, if we want to fail fast if interactive user provides no args and no pipe:
				// We can check if InOrStdin() is actually os.Stdin and if it is a TTY.
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

			cwd, _ := os.Getwd()

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-explain")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf("Please explain the following code concisely:\n\n```\n%s\n```", string(content))

			fmt.Fprintln(cmd.ErrOrStderr(), "Analyzing code...")

			_, err = ag.SendStream(ctx, prompt, func(chunk string) {
				fmt.Fprint(cmd.OutOrStdout(), chunk)
			})

			fmt.Fprintln(cmd.OutOrStdout(), "")

			return err
		},
	}
}

var explainCmd = NewExplainCmd()

func init() {
	rootCmd.AddCommand(explainCmd)
}
