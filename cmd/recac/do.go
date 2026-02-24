package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/utils"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// doExecCommand allows mocking os/exec.Command in tests
var doExecCommand = exec.Command

// DoResponse is the expected JSON structure from the AI agent
type DoResponse struct {
	Command     string `json:"command"`
	Explanation string `json:"explanation"`
}

var doCmd = &cobra.Command{
	Use:   "do [instruction]",
	Short: "Translate natural language to a shell command and execute it",
	Long: `Translate a natural language instruction into a shell command using AI.
The command is explained and you are asked for confirmation before execution.

Example:
  recac do "list all markdown files modified in the last 24 hours"
  recac do "kill the process using port 8080"
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDoCmd,
}

func init() {
	rootCmd.AddCommand(doCmd)
}

func runDoCmd(cmd *cobra.Command, args []string) error {
	instruction := strings.Join(args, " ")

	// Detect Environment
	osName := runtime.GOOS
	var shell string
	if osName == "windows" {
		shell = "cmd"
	} else {
		shell = "sh"
	}

	// Prepare Prompt
	prompt := fmt.Sprintf(`You are a command line expert.
Translate the following natural language instruction into a single executable shell command for %s using %s.

Instruction: "%s"

You MUST return the result as a valid JSON object with the following structure:
{
  "command": "the shell command to execute",
  "explanation": "a brief explanation of what the command does"
}

Do not include any markdown formatting (like `+"```json"+`) around the JSON. Just return the raw JSON string.
`, osName, shell, instruction)

	// Call Agent
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory for testability
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-do")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Thinking...\n")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Robustly extract JSON from the response
	cleanedResp := utils.CleanJSONBlock(resp)

	var result DoResponse
	if err := json.Unmarshal([]byte(cleanedResp), &result); err != nil {
		// Fallback: assume the whole response is the command if JSON fails?
		// No, let's error out or maybe just print the raw response and ask?
		// Better to be safe and error out, or just show the raw response as "Explanation" and ask user to type it.
		// Let's return error for now.
		return fmt.Errorf("failed to parse agent response: %w\nResponse was: %s", err, resp)
	}

	// Display
	fmt.Fprintf(cmd.OutOrStdout(), "\n📝 Explanation: %s\n", result.Explanation)
	fmt.Fprintf(cmd.OutOrStdout(), "🚀 Command: \033[1;32m%s\033[0m\n\n", result.Command)

	// Confirm
	fmt.Fprintf(cmd.OutOrStdout(), "Execute this command? [y/N] ")
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if scanner.Scan() {
		input := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if input == "y" || input == "yes" {
			// Execute
			fmt.Fprintf(cmd.OutOrStdout(), "Running...\n")

			// Use the shell to execute complex commands (pipes, etc)
			var c *exec.Cmd
			if osName == "windows" {
				c = doExecCommand("cmd", "/C", result.Command)
			} else {
				c = doExecCommand("sh", "-c", result.Command)
			}

			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			c.Stdin = cmd.InOrStdin()

			if err := c.Run(); err != nil {
				return fmt.Errorf("command execution failed: %w", err)
			}
			return nil
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
	return nil
}
