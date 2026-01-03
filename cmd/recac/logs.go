package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().String("filter", "", "Filter logs by string match")
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
		filter, _ := cmd.Flags().GetString("filter")

		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}

		file, err := os.Open(logFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to open log file: %v\n", err)
			exit(1)
		}
		defer file.Close()

		reader := bufio.NewReader(file)

		// Helper to process line
		processLine := func(line string) {
			if filter == "" || strings.Contains(line, filter) {
				fmt.Print(line)
			}
		}

		// Initial read
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					if line != "" {
						processLine(line)
					}
					break
				}
				fmt.Fprintf(os.Stderr, "Error reading log file: %v\n", err)
				exit(1)
			}
			processLine(line)
		}

		if follow {
			// Follow mode
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						time.Sleep(500 * time.Millisecond)
						continue
					}
					fmt.Fprintf(os.Stderr, "Error streaming logs: %v\n", err)
					break
				}
				processLine(line)
			}
		}
	},
}
