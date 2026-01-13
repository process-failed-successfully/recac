package main

import (
	"fmt"
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

		// Set the arguments on the root command and execute the 'start' command's logic.
		// This avoids re-executing the binary, which resolves testing issues with flag redefinition.
		root := cmd.Root()
		root.SetArgs(finalArgs)

		return root.Execute()
	},
}
