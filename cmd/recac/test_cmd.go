package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// testExecCommand allows mocking the exec.Command function in tests
var testExecCommand = exec.Command

func NewTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [packages] [flags]",
		Short: "Run tests and auto-diagnose failures",
		Long:  `Runs 'go test' on the specified packages (defaults to ./...).
If tests fail, it captures the output and uses the configured AI agent to analyze the failure and suggest a fix.`,
		DisableFlagParsing: true, // Allow passing flags like -v directly to go test
		RunE: func(cmd *cobra.Command, args []string) error {
			// Since DisableFlagParsing is true, args includes everything after 'test'.
			// e.g. "recac test -v ./..." -> args = ["-v", "./..."]

			testArgs := args
			if len(testArgs) == 0 {
				testArgs = []string{"./..."}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Running tests: go test %s\n", strings.Join(testArgs, " "))

			c := testExecCommand("go", append([]string{"test"}, testArgs...)...)

			// Capture output for analysis
			var outBuf bytes.Buffer
			var errBuf bytes.Buffer

			// Use MultiWriter to stream to stdout/stderr AND capture
			// This gives immediate feedback to the user
			outWriter := io.MultiWriter(cmd.OutOrStdout(), &outBuf)
			errWriter := io.MultiWriter(cmd.ErrOrStderr(), &errBuf)

			c.Stdout = outWriter
			c.Stderr = errWriter

			err := c.Run()

			if err == nil {
				fmt.Fprintln(cmd.OutOrStdout(), "\n✅ All tests passed!")
				return nil
			}

			// Tests failed
			fmt.Fprintln(cmd.ErrOrStderr(), "\n❌ Tests failed. Analyzing with AI...")

			output := outBuf.String() + "\n" + errBuf.String()

			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, _ := os.Getwd()

			// Use the existing factory
			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-test")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			prompt := fmt.Sprintf(`The following 'go test' execution failed.
Analyze the output and explain why it failed.
Then, provide the corrected code block(s) for the file(s) causing the issue.

Test Output:
'''
%s
'''`, output)

			fmt.Fprintln(cmd.ErrOrStderr(), "Consulting Agent...")

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("agent failed to analyze failure: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), "\n--- AI Suggestion ---")
			fmt.Fprintln(cmd.OutOrStdout(), resp)

			return errors.New("tests failed")
		},
	}
	return cmd
}

var testCmd = NewTestCmd()

func init() {
	rootCmd.AddCommand(testCmd)
}
