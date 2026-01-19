package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var diagnoseCmd = &cobra.Command{
	Use:   "diagnose",
	Short: "Diagnose errors using AI",
	Long:  `Executes a command or reads a log file/stdin and asks the AI agent to explain the error and suggest a fix.`,
	Example: `  recac diagnose --command "go test ./..."
  recac diagnose --file error.log
  cat error.log | recac diagnose`,
	RunE: func(cmd *cobra.Command, args []string) error {
		commandStr, _ := cmd.Flags().GetString("command")
		filePath, _ := cmd.Flags().GetString("file")
		logLines, _ := cmd.Flags().GetInt("lines")

		var inputContent string
		var sourceType string

		// 1. Handle Command Execution
		if commandStr != "" {
			sourceType = fmt.Sprintf("output of command '%s'", commandStr)
			fmt.Fprintf(cmd.OutOrStdout(), "Running command: %s\n", commandStr)

			if strings.TrimSpace(commandStr) == "" {
				return errors.New("command cannot be empty")
			}

			// Use sh -c to handle complex commands, pipes, and quotes correctly
			c := exec.Command("sh", "-c", commandStr)
			output, err := c.CombinedOutput()
			inputContent = string(output)

			if err == nil {
				// Command succeeded
				if len(inputContent) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "✅ Command executed successfully with no output.")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "✅ Command executed successfully.")
				// We proceed to diagnose only if there's output and user might want analysis,
				// but usually diagnose is for errors.
				// Let's ask user or just exit?
				// For now, if it succeeds, we assume no diagnosis needed unless force flag (not implemented yet).
				// But maybe success output contains warnings?
				// Let's analyze it anyway if there is output, but preface it.
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "❌ Command failed with error: %v\n", err)
			}
		} else if filePath != "" {
			// 2. Handle File Input
			sourceType = fmt.Sprintf("log file '%s'", filePath)
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", filePath, err)
			}
			inputContent = string(content)
		} else {
			// 3. Handle Stdin
			// Check if we actually have data in stdin
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				sourceType = "stdin"
				content, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				inputContent = string(content)
			} else {
				// No input provided
				return errors.New("please provide a --command, --file, or pipe input via stdin")
			}
		}

		if len(strings.TrimSpace(inputContent)) == 0 {
			return errors.New("input content is empty, nothing to diagnose")
		}

		// Truncate input if too long (keep last N lines)
		lines := strings.Split(inputContent, "\n")
		if len(lines) > logLines {
			lines = lines[len(lines)-logLines:]
			inputContent = strings.Join(lines, "\n")
			fmt.Fprintf(cmd.OutOrStdout(), "(Analyzing last %d lines of output...)\n", logLines)
		}

		// Prepare Agent
		ctx := cmd.Context()
		projectPath, _ := os.Getwd()
		projectName := filepath.Base(projectPath)
		provider := viper.GetString("provider")
		model := viper.GetString("model")

		agentClient, err := agentClientFactory(ctx, provider, model, projectPath, projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		// Construct Prompt
		prompt := fmt.Sprintf(`You are an expert software debugger.
Analyze the following %s and explain the error.
Then, suggest a concrete fix or next steps.

Output:
'''
%s
'''

Analysis:`, sourceType, inputContent)

		fmt.Fprintln(cmd.OutOrStdout(), "\nConsulting Agent...")

		// Stream Response
		_, err = agentClient.SendStream(ctx, prompt, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
		})
		fmt.Fprintln(cmd.OutOrStdout()) // Newline at end

		return err
	},
}

func init() {
	diagnoseCmd.Flags().StringP("command", "c", "", "Shell command to execute and diagnose")
	diagnoseCmd.Flags().StringP("file", "f", "", "Log file to analyze")
	diagnoseCmd.Flags().IntP("lines", "n", 100, "Number of context lines to include (from the end)")
	rootCmd.AddCommand(diagnoseCmd)
}
