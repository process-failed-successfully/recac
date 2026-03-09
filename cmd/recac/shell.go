package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	shellAutoDebug bool
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Start an interactive AI-powered shell",
	Long: `Starts a REPL (Read-Eval-Print Loop) where you can execute shell commands
and use natural language queries (prefixed with '?' or 'ai ') to generate commands.

Features:
- Standard shell execution (supports pipes if executed via shell)
- Natural Language translation: "? find large files" -> "find . -size +100M"
- Auto-Debug: Analyze failed commands with AI (optional)
`,
	RunE: runShell,
}

func init() {
	rootCmd.AddCommand(shellCmd)
	shellCmd.Flags().BoolVar(&shellAutoDebug, "auto-debug", true, "Automatically diagnose failed commands with AI")
}

func runShell(cmd *cobra.Command, args []string) error {
	fmt.Fprintln(cmd.OutOrStdout(), "🚀 RECAC AI Shell")
	fmt.Fprintln(cmd.OutOrStdout(), "Type commands to run, or '? <instruction>' for AI help.")
	fmt.Fprintln(cmd.OutOrStdout(), "Type 'exit' or 'quit' to leave.")
	fmt.Fprintln(cmd.OutOrStdout(), "")

	scanner := bufio.NewScanner(cmd.InOrStdin())
	ctx := context.Background()

	// Detect shell for wrapping
	osName := runtime.GOOS
	var shellBin string
	if osName == "windows" {
		shellBin = "cmd"
	} else {
		// Use user's shell if available
		shellBin = os.Getenv("SHELL")
		if shellBin == "" {
			shellBin = "/bin/sh"
		}
	}

	for {
		// Prompt
		cwd, _ := os.Getwd()
		// Use color for prompt if possible, simplified for now
		fmt.Fprintf(cmd.OutOrStdout(), "recac:%s $ ", cwd)

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())

		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			break
		}

		// Handle AI commands
		if strings.HasPrefix(input, "?") || strings.HasPrefix(input, "ai ") {
			instruction := strings.TrimPrefix(input, "?")
			instruction = strings.TrimPrefix(instruction, "ai ")
			instruction = strings.TrimSpace(instruction)

			generatedCmd, explanation, err := translateShellInstruction(ctx, cmd, instruction, osName, shellBin)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error translating: %v\n", err)
				continue
			}

			fmt.Fprintf(cmd.OutOrStdout(), "📝 %s\n", explanation)
			fmt.Fprintf(cmd.OutOrStdout(), "🚀 Suggested: %s\n", generatedCmd)
			fmt.Fprintf(cmd.OutOrStdout(), "Run this? [Y/n] ")

			if scanner.Scan() {
				confirm := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if confirm == "" || confirm == "y" || confirm == "yes" {
					executeShellLine(ctx, cmd, scanner, generatedCmd, shellBin, osName)
				}
			}
			continue
		}

		// Handle Standard commands
		executeShellLine(ctx, cmd, scanner, input, shellBin, osName)
	}

	return nil
}

func translateShellInstruction(ctx context.Context, cmd *cobra.Command, instruction, osName, shellBin string) (string, string, error) {
	// Reusing logic similar to do.go but refactored
	prompt := fmt.Sprintf(`You are a command line expert.
Translate the following natural language instruction into a single executable shell command for %s using %s.

Instruction: "%s"

You MUST return the result as a valid JSON object with the following structure:
{
  "command": "the shell command to execute",
  "explanation": "a brief explanation"
}

Do not include any markdown formatting. Just return the raw JSON string.`, osName, shellBin, instruction)

	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-shell")
	if err != nil {
		return "", "", err
	}

	fmt.Fprint(cmd.OutOrStdout(), "🤖 Thinking...")
	resp, err := ag.Send(ctx, prompt)
	fmt.Fprintln(cmd.OutOrStdout(), "\r              \r") // Clear "Thinking..."

	if err != nil {
		return "", "", err
	}

	// Clean JSON
	cleanedResp := strings.TrimSpace(resp)
	if strings.HasPrefix(cleanedResp, "```") {
		// strip code blocks
		lines := strings.Split(cleanedResp, "\n")
		if len(lines) >= 2 {
			cleanedResp = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	// Try to find JSON object bounds
	start := strings.Index(cleanedResp, "{")
	end := strings.LastIndex(cleanedResp, "}")
	if start != -1 && end != -1 && start <= end {
		cleanedResp = cleanedResp[start : end+1]
	}

	var result struct {
		Command     string `json:"command"`
		Explanation string `json:"explanation"`
	}
	if err := json.Unmarshal([]byte(cleanedResp), &result); err != nil {
		return "", "", fmt.Errorf("failed to parse AI response: %s", resp)
	}

	return result.Command, result.Explanation, nil
}

func executeShellLine(ctx context.Context, cmd *cobra.Command, scanner *bufio.Scanner, line, shellBin, osName string) {
	// Handle built-in cd
	if strings.HasPrefix(line, "cd ") || line == "cd" {
		args := strings.Fields(line)
		var targetDir string
		if len(args) < 2 {
			// cd to home
			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "cd: %v\n", err)
				return
			}
			targetDir = home
		} else {
			targetDir = args[1]
		}

		// Expand ~ manually if needed (simple case)
		if strings.HasPrefix(targetDir, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				if targetDir == "~" {
					targetDir = home
				} else if strings.HasPrefix(targetDir, "~/") {
					targetDir = filepath.Join(home, targetDir[2:])
				}
			}
		}

		if err := os.Chdir(targetDir); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "cd: %v\n", err)
		}
		return
	}

	var execCmd *exec.Cmd
	if osName == "windows" {
		execCmd = execCommand("cmd", "/C", line)
	} else {
		execCmd = execCommand(shellBin, "-c", line)
	}

	// Connect stdio
	execCmd.Stdin = cmd.InOrStdin()
	execCmd.Stdout = cmd.OutOrStdout()

	// Capture stderr for auto-debug
	var errBuf strings.Builder
	if shellAutoDebug {
		stderrPipe := io.MultiWriter(cmd.ErrOrStderr(), &errBuf)
		execCmd.Stderr = stderrPipe
	} else {
		execCmd.Stderr = cmd.ErrOrStderr()
	}

	err := execCmd.Run()
	if err != nil {
		// Command failed
		if shellAutoDebug {
			fmt.Fprintf(cmd.ErrOrStderr(), "\n❌ Command failed: %v\n", err)

			// Check if we have error output
			errOutput := errBuf.String()
			if errOutput == "" {
				errOutput = err.Error() // fallback
			}

			fmt.Fprint(cmd.OutOrStdout(), "🤖 Diagnose this error? [Y/n] ")

			// Reuse the main loop scanner to capture confirmation
			if scanner.Scan() {
				input := strings.TrimSpace(strings.ToLower(scanner.Text()))
				if input == "" || input == "y" || input == "yes" {
					diagnoseError(ctx, cmd, line, errOutput)
				}
			}
		}
	}
}

func diagnoseError(ctx context.Context, cmd *cobra.Command, command, output string) {
	prompt := fmt.Sprintf(`The following shell command failed:
Command: %s

Error Output:
'''
%s
'''

Explain why it failed and suggest a fix.`, command, output)

	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-shell-debug")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to init agent: %v\n", err)
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Agent failed: %v\n", err)
		return
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n"+resp+"\n")
}
