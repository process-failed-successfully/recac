package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(resumeCmd)
}

var resumeCmd = &cobra.Command{
	Use:   "resume [session-name]",
	Short: "Resume a stopped or errored session",
	Long: `Resumes a session from its last known state. It restores the workspace
and the agent's memory, allowing it to continue from where it left off.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		if session.Status == "running" && sm.IsProcessRunning(session.PID) {
			return fmt.Errorf("session '%s' is already running (PID: %d)", sessionName, session.PID)
		}

		if session.Status != "stopped" && session.Status != "error" && session.Status != "completed" {
			return fmt.Errorf("session '%s' cannot be resumed (status: %s)", sessionName, session.Status)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Resuming session '%s' from workspace: %s\n", sessionName, session.Workspace)

		// Reconstruct the original command arguments, skipping the executable
		originalArgs := session.Command[1:]

		// Filter out any old --name flag and its value
		var argsWithoutName []string
		for i := 0; i < len(originalArgs); i++ {
			if originalArgs[i] == "--name" {
				i++ // Also skip the value
				continue
			}
			argsWithoutName = append(argsWithoutName, originalArgs[i])
		}

		// Add the necessary flags for resumption
		finalArgs := append(
			argsWithoutName,
			"--resume-from", session.Workspace,
			"--name", session.Name, // Ensure the session name is preserved
		)

		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		// Replace the current process with the new command
		// Note: syscall.Exec is not available in all Go environments (e.g., Windows)
		// and might be overkill. A sub-process is safer and more portable.

		newCmd := execCommand(executable, finalArgs...)
		newCmd.Stdout = os.Stdout
		newCmd.Stderr = os.Stderr
		newCmd.Stdin = os.Stdin

		// We must not use Start() and Wait() here, because we want the new process to take over.
		// In a Unix-like environment, we could use syscall.Exec.
		// For portability, we will run it as a subprocess and exit.
		if err := newCmd.Run(); err != nil {
			return fmt.Errorf("failed to start resumed session: %w", err)
		}

		return nil
	},
}
