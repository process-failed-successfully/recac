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
	Short: "Stop a running session",
	Long:  `Stop a running session gracefully. If no session name is provided, it lists running sessions and prompts for a selection.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := newSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		var sessionName string
		if len(args) == 0 {
			sessionName, err = selectSessionToStop(sm)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				exit(1)
			}
			if sessionName == "" {
				// No session selected or no running sessions
				return
			}
		} else {
			sessionName = args[0]
		}

		if err := runner.StopSessionFunc(sm, sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		fmt.Printf("Session '%s' stopped successfully\n", sessionName)
	},
}

func selectSessionToStop(sm *runner.SessionManager) (string, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return "", fmt.Errorf("could not list sessions: %w", err)
	}

	var runningSessions []*runner.SessionState
	for _, s := range sessions {
		if s.Status == "running" {
			runningSessions = append(runningSessions, s)
		}
	}

	if len(runningSessions) == 0 {
		fmt.Println("No running sessions to stop.")
		return "", nil
	}

	fmt.Println("Select a session to stop:")
	for i, s := range runningSessions {
		fmt.Printf("%d: %s (PID: %d)\n", i+1, s.Name, s.PID)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("Selection cannot be empty.")
			continue
		}

		selection, err := strconv.Atoi(input)
		if err != nil || selection < 1 || selection > len(runningSessions) {
			fmt.Println("Invalid selection. Please enter a number from the list.")
			continue
		}

		return runningSessions[selection-1].Name, nil
	}
}
