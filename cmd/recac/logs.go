package main

import (
	"bufio"
	"fmt"
	"os"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs [session-name]",
	Short: "View session logs",
	Long:  `View logs for a specific session. Use --follow to stream logs in real-time.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]
		follow := cmd.Flags().Lookup("follow").Changed

		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			os.Exit(1)
		}

		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		file, err := os.Open(logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to open log file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "Error reading log file: %v\n", err)
			os.Exit(1)
		}

		if follow {
			// For follow mode, we would need to implement tail -f functionality
			// For now, just print existing logs
			fmt.Println("\n(Follow mode not yet implemented - showing current logs)")
		}
	},
}
