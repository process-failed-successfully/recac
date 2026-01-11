
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/nxadm/tail"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output")
}

var logsCmd = &cobra.Command{
	Use:   "logs [SESSION_NAME]",
	Short: "Fetch the logs of a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		follow, _ := cmd.Flags().GetBool("follow")

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			return err
		}

		if _, err := os.Stat(logFile); os.IsNotExist(err) {
			return fmt.Errorf("log file for session '%s' not found at %s", sessionName, logFile)
		}

		if follow {
			return tailLog(cmd, logFile)
		}
		return printLog(cmd, logFile)
	},
}

func printLog(cmd *cobra.Command, logFile string) error {
	file, err := os.Open(logFile)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(cmd.OutOrStdout(), file)
	return err
}

func tailLog(cmd *cobra.Command, logFile string) error {
	t, err := tail.TailFile(logFile, tail.Config{
		Follow:    true,
		MustExist: true,
		Poll:      true, // Use polling to work around potential inotify issues in some environments
		ReOpen:    true,
	})
	if err != nil {
		return fmt.Errorf("failed to start tailing log file: %w", err)
	}

	for line := range t.Lines {
		fmt.Fprintln(cmd.OutOrStdout(), line.Text)
	}

	return t.Wait()
}
