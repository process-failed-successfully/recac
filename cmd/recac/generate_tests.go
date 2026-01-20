package main

import (
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

func NewGenerateTestsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "generate-tests [file]",
		Aliases: []string{"gen-tests", "test-gen"},
		Short:   "Generate unit tests for a file with optional auto-repair",
		Long: `Reads a file and uses the configured AI agent to generate unit tests for it.
Can optionally run the tests and attempt to fix them if they fail (requires --output and --auto-fix).`,
		Args: cobra.MaximumNArgs(1),
		RunE: runTestGenLoop,
	}

	cmd.Flags().StringP("framework", "f", "", "Testing framework to use (e.g., testing, pytest, jest)")
	cmd.Flags().StringP("output", "o", "", "Write output to file (required for auto-fix)")
	cmd.Flags().Bool("auto-fix", false, "Automatically run tests and fix failures")
	cmd.Flags().Int("max-retries", 3, "Maximum number of repair attempts")
	cmd.Flags().String("run-cmd", "", "Command to run tests (e.g., 'go test ./pkg/...'). If empty, tries to infer.")

	return cmd
}

func runTestGenLoop(cmd *cobra.Command, args []string) error {
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

	outputFile, _ := cmd.Flags().GetString("output")
	autoFix, _ := cmd.Flags().GetBool("auto-fix")

	if autoFix && outputFile == "" {
		return errors.New("--auto-fix requires --output to be set")
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

	// Initial Prompt
	prompt := fmt.Sprintf("Please generate unit tests for the following code (file: %s).\n", fileName)
	if framework != "" {
		prompt += fmt.Sprintf("Use the '%s' testing framework.\n", framework)
	} else {
		prompt += "Infer the best testing framework for the language.\n"
	}
	prompt += "Return the test code. If possible, enclose it in a markdown code block.\n\n"
	prompt += fmt.Sprintf("```\n%s\n```", string(content))

	fmt.Fprintf(cmd.ErrOrStderr(), "Generating tests for %s...\n", fileName)

	// Loop
	maxRetries, _ := cmd.Flags().GetInt("max-retries")
	if !autoFix {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nAttempt %d/%d to fix tests...\n", attempt, maxRetries)
		}

		// Generate
		response, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		code := extractCodeBlock(response)

		// Output
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(code), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Tests written to %s\n", outputFile)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), code)
			// If no output file, we can't run tests, so we are done.
			return nil
		}

		if !autoFix {
			return nil
		}

		// Run Tests
		runCmd, _ := cmd.Flags().GetString("run-cmd")
		if runCmd == "" {
			// Infer
			if strings.HasSuffix(fileName, ".go") || strings.HasSuffix(outputFile, ".go") {
				// For Go, we usually want to run the test in the current package
				// But args are needed. "go test <outputFile>" works if standalone,
				// but usually "go test" in dir is better.
				// Since we wrote the file, "go test" in the current directory should pick it up.
				runCmd = "go test -v ."
			} else if strings.HasSuffix(fileName, ".py") {
				runCmd = "pytest " + outputFile
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Cannot infer test command. Please use --run-cmd.")
				return nil
			}
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Running tests: %s\n", runCmd)
		output, err := executeShellCommand(runCmd)
		if err == nil {
			fmt.Fprintln(cmd.ErrOrStderr(), "✅ Tests passed!")
			return nil
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "❌ Tests failed:\n%s\n", output)

		// Prepare prompt for next iteration
		prompt = fmt.Sprintf("The tests you generated failed with the following error:\n\n```\n%s\n```\n\nPlease fix the tests. Return the full fixed test code.", output)
	}

	return fmt.Errorf("failed to generate passing tests after %d attempts", maxRetries)
}

var executeShellCommand = func(command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", errors.New("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
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
