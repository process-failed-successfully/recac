package main

import (
	"errors"
	"fmt"
	"os"
	"recac/internal/runner"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop [session-name]",
	Short: "Stop a running session",
	Long:  `Stop a running session gracefully. Sends SIGTERM first, then SIGKILL if needed.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionName string
		var err error

		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		if len(args) == 0 {
			sessionName, err = interactiveSessionSelect(sm)
			if err != nil {
				return err
			}
		} else {
			sessionName = args[0]
		}

		if err := sm.StopSession(sessionName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' stopped successfully\n", sessionName)
		return nil
	},
}

var surveyAskOne = survey.AskOne

// interactiveSessionSelect prompts the user to select a running session.
func interactiveSessionSelect(sm runner.ISessionManager) (string, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return "", fmt.Errorf("could not list sessions: %w", err)
	}

	var runningSessions []string
	for _, s := range sessions {
		if s.Status == "running" {
			runningSessions = append(runningSessions, s.Name)
		}
	}

	if len(runningSessions) == 0 {
		return "", errors.New("no running sessions to stop")
	}

	var selectedSession string
	prompt := &survey.Select{
		Message: "Choose a session to stop:",
		Options: runningSessions,
	}
	if err := surveyAskOne(prompt, &selectedSession, survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)); err != nil {
		return "", fmt.Errorf("failed to get user input: %w", err)
	}

	return selectedSession, nil
}
