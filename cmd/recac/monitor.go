package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// monitorExecCommand allows mocking os/exec.Command in tests
var monitorExecCommand = exec.Command

var monitorCmd = &cobra.Command{
	Use:   "monitor [command]...",
	Short: "Monitor a command and explain errors in real-time",
	Long: `Executes a shell command and monitors its output for errors.
If an error pattern (e.g., "Error:", "panic:", "FAIL") is detected, it uses the configured AI agent
to explain the error and suggest a fix without interrupting the command.

Example:
  recac monitor npm start
  recac monitor go test -v ./...
`,
	DisableFlagParsing: true,
	RunE:               runMonitor,
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

func runMonitor(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command provided")
	}

	commandName := args[0]
	commandArgs := args[1:]

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	// Prepare command
	execCmd := monitorExecCommand(commandName, commandArgs...)
	execCmd.Stdin = cmd.InOrStdin()

	// Rolling buffer (lines)
	// We'll use a slice and append, truncating if too large.
	// Or a ring buffer. Simple slice is fine for now.
	bufferSize := 50
	logBuffer := make([]string, 0, bufferSize)
	var bufferMu sync.Mutex

	// Regex triggers (simple string contains for now to save deps/perf)
	triggers := viper.GetStringSlice("monitor.triggers")
	if len(triggers) == 0 {
		triggers = []string{
			"error:", "Error:", "ERROR:",
			"panic:", "Panic:", "PANIC:",
			"fail:", "Fail:", "FAIL:",
			"exception:", "Exception:", "EXCEPTION:",
			"fatal:", "Fatal:", "FATAL:",
		}
	}

	// Debounce
	lastTrigger := time.Time{}
	debounceDuration := 5 * time.Second

	// Channels for output processing
	// We use io.Pipe to create a reader that we can scan line by line
	// while also writing to the user output.
	stdoutR, stdoutW := io.Pipe()
	stderrR, stderrW := io.Pipe()

	execCmd.Stdout = io.MultiWriter(cmd.OutOrStdout(), stdoutW)
	execCmd.Stderr = io.MultiWriter(cmd.ErrOrStderr(), stderrW)

	// WaitGroup for scanning goroutines
	var wg sync.WaitGroup
	wg.Add(2)

	// WaitGroup for AI goroutines
	var aiWg sync.WaitGroup

	// Helper to process lines
	processScanner := func(scanner *bufio.Scanner, source string) {
		defer wg.Done()
		for scanner.Scan() {
			line := scanner.Text()

			// Update buffer
			bufferMu.Lock()
			if len(logBuffer) >= bufferSize {
				logBuffer = logBuffer[1:]
			}
			logBuffer = append(logBuffer, line)
			currentBuffer := make([]string, len(logBuffer))
			copy(currentBuffer, logBuffer)
			bufferMu.Unlock()

			// Check for triggers
			triggered := false
			for _, t := range triggers {
				if strings.Contains(line, t) {
					triggered = true
					break
				}
			}

			if triggered {
				bufferMu.Lock()
				now := time.Now()
				if now.Sub(lastTrigger) > debounceDuration {
					lastTrigger = now
					bufferMu.Unlock()

					// Trigger AI in a separate goroutine
					aiWg.Add(1)
					go func(buf []string, triggerLine string) {
						defer aiWg.Done()
						explainError(ctx, cmd, buf, triggerLine, args)
					}(currentBuffer, line)
				} else {
					bufferMu.Unlock()
				}
			}
		}
	}

	go processScanner(bufio.NewScanner(stdoutR), "stdout")
	go processScanner(bufio.NewScanner(stderrR), "stderr")

	// Run command
	err := execCmd.Run()

	// Close pipes to stop scanners
	stdoutW.Close()
	stderrW.Close()
	wg.Wait()
	aiWg.Wait() // Wait for any pending AI explanations

	if err != nil {
		// If command failed but we haven't triggered recently (or maybe at all for exit code failure),
		// we could trigger one last time.
		// For now, let's rely on the stream triggers.
		return err
	}

	return nil
}

func explainError(ctx context.Context, cmd *cobra.Command, logs []string, triggerLine string, args []string) {
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-monitor")
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n[RECAC] Failed to create agent: %v\n", err)
		return
	}

	fullCommand := strings.Join(args, " ")
	logContent := strings.Join(logs, "\n")

	prompt := fmt.Sprintf(`I am monitoring the command: "%s"
I detected a potential error:
"%s"

Recent logs:
%s

Please analyze the error and suggest a fix. Be concise.
`, fullCommand, triggerLine, logContent)

	fmt.Fprintf(cmd.OutOrStdout(), "\n\n🤖 [RECAC] Analyzing error...\n")

	// Use Stream?
	var responseBuf bytes.Buffer
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
		responseBuf.WriteString(chunk)
	})

	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "\n[RECAC] Agent failed: %v\n", err)
	}
}
