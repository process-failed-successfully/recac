package main

import (
	"fmt"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop [session-name]",
	Short: "Stop a running session",
	Long:  `Stop a running session gracefully. Sends SIGTERM first, then SIGKILL if needed.`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := newSessionManager()
		if err != nil {
			cmd.PrintErrf("Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		if len(args) == 1 {
			stopNamedSession(cmd, sm, args[0])
		} else {
			stopInteractive(cmd, sm)
		}
	},
}

func stopNamedSession(cmd *cobra.Command, sm ISessionManager, sessionName string) {
	if err := sm.StopSession(sessionName); err != nil {
		cmd.PrintErrf("Error: %v\n", err)
		exit(1)
	}

	cmd.Printf("Session '%s' stopped successfully\n", sessionName)
}

func stopInteractive(cmd *cobra.Command, sm ISessionManager) {
	sessions, err := sm.ListSessions()
	if err != nil {
		cmd.PrintErrf("Error: failed to list sessions: %v\n", err)
		exit(1)
	}

	var runningSessions []*runner.SessionState
	for _, s := range sessions {
		if s.Status == "running" {
			runningSessions = append(runningSessions, s)
		}
	}

	if len(runningSessions) == 0 {
		cmd.Println("No running sessions to stop.")
		return
	}

	cmd.Println("Running sessions:")
	for i, s := range runningSessions {
		cmd.Printf("%d: %s (PID: %d)\n", i+1, s.Name, s.PID)
	}

	cmd.Print("Enter the number of the session to stop: ")
	var choice int
	// Use Fscanf to read from the command's input stream
	_, err = fmt.Fscanf(cmd.InOrStdin(), "%d", &choice)
	if err != nil {
		cmd.PrintErrf("Error: invalid input\n")
		exit(1)
	}

	if choice < 1 || choice > len(runningSessions) {
		cmd.PrintErrf("Error: invalid session number\n")
		exit(1)
	}

	sessionToStop := runningSessions[choice-1]
	stopNamedSession(cmd, sm, sessionToStop.Name)
}
