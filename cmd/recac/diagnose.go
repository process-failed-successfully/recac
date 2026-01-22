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

func NewDiagnoseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diagnose [file]",
		Short: "Analyze crash logs or stack traces using AI",
		Long: `Reads a log file or stack trace (from file or stdin) and uses the configured AI agent to diagnose the issue.
It automatically extracts referenced files from the local codebase to provide context to the AI.`,
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
				// Check for stdin
				in := cmd.InOrStdin()
				// Basic check if input is available (especially for interactive usage)
				// Note: In tests we mock InOrStdin, so we just read from it.
				// In real usage, if user just types "recac diagnose" without pipe,
				// it might hang waiting for input if we don't check.
				// But standard unix behavior is to read from stdin.
				// We can try to be helpful if it's a TTY.
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

			logContent := string(content)

			// Extract context
			fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning for referenced files...")
			fileContexts, err := extractFileContexts(logContent)
			if err != nil {
				// Don't fail hard, just warn. The AI might still be able to help without context.
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not extract file contexts: %v\n", err)
				fileContexts = "No local files could be linked to the stack trace."
			}

			// Construct prompt
			prompt := fmt.Sprintf(`I have a crash log or stack trace.
Please diagnose the issue and suggest a fix.

<log_or_trace>
%s
</log_or_trace>

<referenced_files>
%s
</referenced_files>

Explain the root cause and provide a corrected code snippet if possible.
`, logContent, fileContexts)

			// Call Agent
			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, _ := os.Getwd()

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-diagnose")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "🤖 Diagnosing with AI...")
			_, err = ag.SendStream(ctx, prompt, func(chunk string) {
				fmt.Fprint(cmd.OutOrStdout(), chunk)
			})
			fmt.Fprintln(cmd.OutOrStdout(), "")

			return err
		},
	}
}

var diagnoseCmd = NewDiagnoseCmd()

func init() {
	rootCmd.AddCommand(diagnoseCmd)
}
