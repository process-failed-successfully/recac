package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/utils"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	healCommand string
	healRetries int
)

var healCmd = &cobra.Command{
	Use:   "heal",
	Short: "Automatically fix build and test failures",
	Long: `Iteratively runs a command (e.g., "go test ./...") and uses AI to fix failures.
It parses the output for errors, identifies relevant files, and asks the agent to patch them.`,
	RunE: runHeal,
}

func init() {
	rootCmd.AddCommand(healCmd)
	healCmd.Flags().StringVarP(&healCommand, "command", "c", "go test ./...", "Command to run and fix")
	healCmd.Flags().IntVarP(&healRetries, "retries", "r", 3, "Maximum number of fix attempts")
}

func runHeal(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	for i := 0; i <= healRetries; i++ {
		fmt.Fprintf(cmd.OutOrStdout(), "🔄 Attempt %d/%d: Running '%s'...\n", i+1, healRetries+1, healCommand)

		// Run the command
		output, err := runCommand(healCommand)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Command succeeded!")
			return nil
		}

		// Command failed
		fmt.Fprintf(cmd.ErrOrStderr(), "❌ Command failed:\n%s\n", output)

		if i == healRetries {
			return fmt.Errorf("failed to heal after %d retries", healRetries)
		}

		// Identify failed files
		files := extractFailedFiles(output)
		if len(files) == 0 {
			// Fallback: If no files identified from output, maybe just ask agent with the whole output?
			// But we need files to provide context.
			// Let's rely on extracting files for now.
			return fmt.Errorf("command failed but no specific files were identified in the output to fix")
		}

		fmt.Fprintf(cmd.OutOrStdout(), "🔍 Identified relevant files: %v\n", files)

		// Read file contents
		fileContents := make(map[string]string)
		for _, f := range files {
			content, err := os.ReadFile(f)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to read %s: %v\n", f, err)
				continue
			}
			fileContents[f] = string(content)
		}

		if len(fileContents) == 0 {
			return fmt.Errorf("could not read any relevant files to fix")
		}

		// Prepare prompt
		var promptBuilder strings.Builder
		promptBuilder.WriteString(fmt.Sprintf("The command '%s' failed with the following output:\n", healCommand))
		promptBuilder.WriteString("```\n")
		// Truncate output if too long?
		if len(output) > 5000 {
			// Keep head and tail to capture initial command echo and final error summary
			promptBuilder.WriteString(output[:1000])
			promptBuilder.WriteString("\n...(truncated)...\n")
			promptBuilder.WriteString(output[len(output)-4000:])
		} else {
			promptBuilder.WriteString(output)
		}
		promptBuilder.WriteString("\n```\n\n")
		promptBuilder.WriteString("Please fix the errors in the following files. Return the FULL content of the fixed files wrapped in <file path=\"...\"> tags.\n\n")

		for path, content := range fileContents {
			promptBuilder.WriteString(fmt.Sprintf("<file path=\"%s\">\n%s\n</file>\n\n", path, content))
		}

		// Get Agent
		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-heal")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "🧠 Consulting agent for fixes...")
		resp, err := ag.Send(ctx, promptBuilder.String())
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		// Apply fixes
		patches := utils.ParseFileBlocks(resp)
		if len(patches) == 0 {
			fmt.Fprintln(cmd.ErrOrStderr(), "⚠️  Agent did not return any file blocks.")
			continue
		}

		for path, content := range patches {
			fmt.Fprintf(cmd.OutOrStdout(), "📝 Applying fix to %s...\n", path)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write file %s: %w", path, err)
			}
		}
	}

	return nil
}

func runCommand(command string) (string, error) {
	// Naive splitting handles spaces but not quotes.
	// For "go test ./...", it's fine.
	// For "go test -v 'foo bar'", it might break.
	// Using sh -c is safer for complex commands.
	cmd := exec.Command("sh", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func extractFailedFiles(output string) []string {
	// Common pattern: path/to/file.go:line:col
	// We capture the file path.
	// Allow leading whitespace for indented test output.
	re := regexp.MustCompile(`(?m)^\s*([a-zA-Z0-9_\-\./\\]+\.[a-zA-Z0-9]+):\d+`)
	matches := re.FindAllStringSubmatch(output, -1)

	unique := make(map[string]bool)
	var files []string

	for _, m := range matches {
		if len(m) >= 2 {
			f := m[1]
			// Check if file exists to reduce false positives
			if _, err := os.Stat(f); err == nil {
				if !unique[f] {
					unique[f] = true
					files = append(files, f)
				}
			}
		}
	}
	return files
}
