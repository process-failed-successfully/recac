package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debugCmd = &cobra.Command{
	Use:   "debug [command]",
	Short: "Execute a command and diagnose failures with AI",
	Long: `Executes the provided command (in a shell).
If the command fails, it captures the output and uses the configured AI agent to analyze the error, scan referenced files, and suggest a fix.

Example:
  recac debug "go test ./pkg/..."
  recac debug "npm run build"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Join args to form the full command string
		commandStr := strings.Join(args, " ")
		fmt.Printf("Running: %s\n", commandStr)

		// Parse the command string to properly handle quotes and spaces
		parts, err := shellquote.Split(commandStr)
		if err != nil {
			return fmt.Errorf("failed to parse command: %w", err)
		}
		if len(parts) == 0 {
			return fmt.Errorf("empty command")
		}

		// Execute command using parsed parts
		var execCmd = exec.Command(parts[0], parts[1:]...)

		// Capture stdout and stderr together
		var outputBuf bytes.Buffer
		execCmd.Stdout = &outputBuf
		execCmd.Stderr = &outputBuf

		err = execCmd.Run()
		output := outputBuf.String()

		// Print the output to the user so they see what happened
		fmt.Fprint(cmd.OutOrStdout(), output)

		if err == nil {
			// Command succeeded
			return nil
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "\n❌ Command failed with error: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing failure with AI...")

		// Scan for file references in the output
		fileContexts, scanErr := extractFileContexts(output)
		if scanErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to scan files: %v\n", scanErr)
		}

		// Construct Prompt
		prompt := fmt.Sprintf(`The following command failed:
Command: %s

Error Output:
'''
%s
'''

Context Files:
%s

Please analyze the error and the provided context. Suggest a specific fix or explain why it failed.
`, commandStr, output, fileContexts)

		// Call Agent
		ctx := context.Background()
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		cwd, _ := os.Getwd()

		ag, agentErr := agentClientFactory(ctx, provider, model, cwd, "recac-debug")
		if agentErr != nil {
			return fmt.Errorf("failed to create agent: %w", agentErr)
		}

		_, agentErr = ag.SendStream(ctx, prompt, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
		})
		fmt.Println("")

		return agentErr
	},
}

func init() {
	rootCmd.AddCommand(debugCmd)
}
