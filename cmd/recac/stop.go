package main

import (
	"bufio"
	"fmt"
	"os"
	"recac/internal/runner"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop [session-name]",
	Short: "Stop a running session, interactively if no name is provided",
	Long:  `Stop a running session gracefully. If a session-name is provided, it stops the specified session. If no session-name is provided, it lists active sessions and prompts the user to choose which one to stop.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := newSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var sessionName string
		if len(args) == 1 {
			sessionName = args[0]
		} else {
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			var activeSessions []*runner.SessionState
			for _, s := range sessions {
				// A simple check for a running process
				if s.PID > 0 && (s.Status == "RUNNING" || s.Status == "UNKNOWN") {
					activeSessions = append(activeSessions, s)
				}
			}

			if len(activeSessions) == 0 {
				fmt.Println("No active sessions to stop.")
				return nil
			}

			fmt.Println("Select a session to stop:")
			for i, s := range activeSessions {
				fmt.Printf("%d: %s (PID: %d, Status: %s)\n", i+1, s.Name, s.PID, s.Status)
			}

			reader := bufio.NewReader(os.Stdin)
			fmt.Print("> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read selection: %w", err)
			}
			input = strings.TrimSpace(input)
			choice, err := strconv.Atoi(input)
			if err != nil || choice < 1 || choice > len(activeSessions) {
				return fmt.Errorf("invalid selection")
			}
			sessionName = activeSessions[choice-1].Name
		}

		if err := sm.StopSession(sessionName); err != nil {
			return err
		}

		fmt.Printf("Session '%s' stopped successfully\n", sessionName)
		return nil
	},
}
