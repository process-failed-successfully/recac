package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var troubleshootCmd = &cobra.Command{
	Use:   "troubleshoot [command]",
	Short: "Interactively troubleshoot and fix command failures",
	Long: `Runs a command and, if it fails, uses an AI agent to analyze the output,
explain the error, and propose a fix. You can interactively apply the fix and re-run the command.`,
	Args: cobra.ExactArgs(1),
	RunE: runTroubleshoot,
}

func init() {
	rootCmd.AddCommand(troubleshootCmd)
}

func runTroubleshoot(cmd *cobra.Command, args []string) error {
	commandToRun := args[0]
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Use a shared reader for the session to handle buffering correctly
	inputReader := bufio.NewReader(cmd.InOrStdin())

	for {
		fmt.Fprintf(cmd.OutOrStdout(), "\n🚀 Running: %s\n", commandToRun)
		fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------")

		output, exitCode, err := executeCommandAndStream(cmd, commandToRun)
		if err != nil {
			// This is an execution error (e.g. command not found), not necessarily a non-zero exit code
			return fmt.Errorf("failed to execute command: %w", err)
		}

		if exitCode == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "\n✅ Command succeeded!")
			return nil
		}

		fmt.Fprintf(cmd.OutOrStdout(), "\n❌ Command failed (exit code %d).\n", exitCode)

		if !askUserYesNo(cmd, inputReader, "Troubleshoot with AI? [Y/n]", true) {
			return fmt.Errorf("command failed")
		}

		// AI Analysis
		fmt.Fprintln(cmd.OutOrStdout(), "\n🧠 Analyzing failure...")

		// 1. Extract context
		fileContexts, err := extractFileContexts(output)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not extract file contexts: %v\n", err)
			fileContexts = "No local files could be linked to the output."
		}

		// 2. Prepare Agent
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		cwd, _ := os.Getwd()

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-troubleshoot")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		// 3. Prompt
		prompt := fmt.Sprintf(`The command '%s' failed. Please analyze the output and the referenced code to fix the failure.

<command_output>
%s
</command_output>

<referenced_code>
%s
</referenced_code>

INSTRUCTIONS:
1. Explain the cause of the error concisely.
2. Provide the corrected code for the affected file(s).
3. Return the FULL CONTENT of the modified file(s) wrapped in <file path="...">...</file> tags.
   Example:
   <file path="pkg/math/sum.go">
   package math
   ...
   </file>

If no code changes are needed (e.g., environmental issue), just explain the fix.
`, commandToRun, output, fileContexts)

		// 4. Send and Stream
		var respBuilder strings.Builder
		fmt.Fprintln(cmd.OutOrStdout(), "")
		resp, err := ag.SendStream(ctx, prompt, func(chunk string) {
			fmt.Fprint(cmd.OutOrStdout(), chunk)
			respBuilder.WriteString(chunk)
		})
		fmt.Fprintln(cmd.OutOrStdout(), "")

		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		// 5. Parse Fixes
		fullResp := resp // SendStream might return full response or we use builder. usually SendStream returns full string.
		// Wait, ag.SendStream signature?
		// Checking diagnose.go: _, err = ag.SendStream(...)
		// It returns (string, error).

		files := utils.ParseFileBlocks(fullResp)
		if len(files) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "ℹ️  No code changes proposed.")
			if !askUserYesNo(cmd, inputReader, "Try again? [y/N]", false) {
				return fmt.Errorf("command failed")
			}
			continue // Retry loop? Or just exit? User might want to fix manually and retry.
		}

		// 6. Review and Apply
		fmt.Fprintln(cmd.OutOrStdout(), "\n📝 Proposed Fixes:")
		for path, newContent := range files {
			fmt.Fprintf(cmd.OutOrStdout(), "--- %s ---\n", path)
			// Read original for diff
			originalContent, err := os.ReadFile(path)
			if err == nil {
				diff, _ := utils.GenerateDiff(path, string(originalContent), newContent)
				fmt.Fprintln(cmd.OutOrStdout(), diff)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "(New File)")
				fmt.Fprintln(cmd.OutOrStdout(), newContent)
			}
		}

		if askUserYesNo(cmd, inputReader, "Apply these fixes? [y/N]", false) {
			for path, content := range files {
				if err := writeFileFunc(path, []byte(content), 0644); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Failed to write %s: %v\n", path, err)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", path)
				}
			}

			if askUserYesNo(cmd, inputReader, "Rerun command? [Y/n]", true) {
				continue
			}
			return nil
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Fixes discarded.")
			if askUserYesNo(cmd, inputReader, "Try again (rerun)? [y/N]", false) {
				continue
			}
			return fmt.Errorf("command failed")
		}
	}
}

func executeCommandAndStream(cmd *cobra.Command, command string) (string, int, error) {
	// Use sh -c to allow complex commands with pipes/args
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", 0, fmt.Errorf("empty command")
	}

	// We want to run it via shell to support 'go test ./...' expansion if needed,
	// but exec.Command does not do expansion unless we use sh.
	// However, users might pass "go test ./..." as a single arg string to this tool.
	// `recac troubleshoot "go test ./..."`

	execCmd := execCommand("sh", "-c", command)

	stdoutPipe, err := execCmd.StdoutPipe()
	if err != nil {
		return "", 0, err
	}
	stderrPipe, err := execCmd.StderrPipe()
	if err != nil {
		return "", 0, err
	}

	if err := execCmd.Start(); err != nil {
		return "", 0, err
	}

	var outputBuf strings.Builder
	var mu sync.Mutex
	done := make(chan bool)

	// Stream stdout
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(cmd.OutOrStdout(), line)
			mu.Lock()
			outputBuf.WriteString(line + "\n")
			mu.Unlock()
		}
		done <- true
	}()

	// Stream stderr
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(cmd.ErrOrStderr(), line)
			mu.Lock()
			outputBuf.WriteString(line + "\n")
			mu.Unlock()
		}
		done <- true
	}()

	err = execCmd.Wait()
	<-done
	<-done

	exitCode := 0
	if err != nil {
		// Try to get exit code
		// ExitError is returned if command exited with non-zero
		if exitErr, ok := err.(*os.PathError); ok {
			return outputBuf.String(), 0, exitErr
		}
		// In Go, Wait() returns an error if exit code != 0.
		// We can assume exit code 1 if generic error, or try to cast to ExitError
		// But for our purpose, err != nil is enough to know it failed.
		// We'll try to extract the code if possible, default to 1.
		exitCode = 1
		// If using os/exec, we can't easily get the exact code without casting to syscall.WaitStatus which is platform specific.
		// Cobra/Go simplifies this usually.
		// Let's just say if err != nil, exitCode = 1.
	}

	return outputBuf.String(), exitCode, nil
}

func askUserYesNo(cmd *cobra.Command, reader *bufio.Reader, prompt string, defaultYes bool) bool {
	fmt.Fprint(cmd.OutOrStdout(), prompt+" ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}

	return input == "y" || input == "yes"
}
