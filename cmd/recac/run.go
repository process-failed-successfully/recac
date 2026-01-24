package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// runExecCommand allows mocking os/exec.Command in tests
var runExecCommand = exec.Command

var runCmd = &cobra.Command{
	Use:   "run [command]...",
	Short: "Execute a shell command and ask AI for help if it fails",
	Long: `Execute a shell command. If the command fails (returns a non-zero exit code),
it captures the output and uses the configured AI agent to explain the error and suggest a fix.

Example:
  recac run make build
  recac run -- go test ./...
`,
	DisableFlagParsing: true, // Allow flags to be passed to the subcommand
	RunE:               executeRunCmd,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func executeRunCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided")
	}

	// 1. Prepare command
	commandName := args[0]
	commandArgs := args[1:]
	execCmd := runExecCommand(commandName, commandArgs...)

	// 2. Setup IO capture
	// We want to stream to the user AND capture for the AI
	var stdoutBuf, stderrBuf bytes.Buffer

	// Use cmd.OutOrStdout/ErrOrStderr for testability
	userStdout := cmd.OutOrStdout()
	userStderr := cmd.ErrOrStderr()

	execCmd.Stdout = io.MultiWriter(userStdout, &stdoutBuf)
	execCmd.Stderr = io.MultiWriter(userStderr, &stderrBuf)
	execCmd.Stdin = cmd.InOrStdin()

	// 3. Run command
	// We don't print "Running..." to avoid polluting the output if it's a script
	err := execCmd.Run()

	// 4. Handle success
	if err == nil {
		return nil
	}

	// 5. Handle failure
	// Only intervene if it's an exit error
	if _, ok := err.(*exec.ExitError); !ok {
		// If it's not an exit error (e.g. binary not found), we still might want help,
		// but let's just return the error for now as it prints to stderr usually.
		return err
	}

	fmt.Fprintf(userStderr, "\n\n❌ Command failed. Asking AI for help...\n\n")

	// 6. Consult AI
	// Get context
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use cmd context if available, fallback to background
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	agent, err := agentClientFactory(ctx, provider, model, cwd, "recac-run")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Reconstruct command with basic quoting for clarity in prompt
	var quotedArgs []string
	for _, arg := range args {
		if strings.ContainsAny(arg, " \t\n\"'") {
			quotedArgs = append(quotedArgs, fmt.Sprintf("%q", arg))
		} else {
			quotedArgs = append(quotedArgs, arg)
		}
	}
	fullCommand := strings.Join(quotedArgs, " ")
	output := stdoutBuf.String() + "\n" + stderrBuf.String()

	prompt := fmt.Sprintf(`I ran the following command and it failed:

<command>
%s
</command>

<output>
%s
</output>

Please explain why it failed and suggest a fix.
If the fix involves a corrected command, provide it clearly.
`, fullCommand, output)

	_, err = agent.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(userStdout, chunk)
	})
	fmt.Fprintln(userStdout, "") // Ensure newline at end

	if err != nil {
		return fmt.Errorf("agent failed to respond: %w", err)
	}

	// We still return the original error so the exit code is propagated if this was part of a script
	// But recac might catch it.
	// The Cobra/Main wrapper usually exits 1 on error.
	return fmt.Errorf("command execution failed")
}
