package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var searchLogsCmd = &cobra.Command{
	Use:   "search-logs [pattern]",
	Short: "Search for a pattern in all session logs",
	Long: `Scans through all session log files and prints lines that match the provided pattern.
Each matching line is prefixed with the session name for context.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]
		return searchLogs(pattern, cmd)
	},
}

func init() {
	rootCmd.AddCommand(searchLogsCmd)
}

func searchLogs(pattern string, cmd *cobra.Command) error {
	sm, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	foundMatch := false
	for _, session := range sessions {
		logPath := filepath.Join(sm.SessionsDir(), session.Name+".log")
		file, err := os.Open(logPath)
		if err != nil {
			// Log file might not exist for some sessions, so we skip it.
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, pattern) {
				cmd.Println(fmt.Sprintf("[%s] %s", session.Name, line))
				foundMatch = true
			}
		}
		file.Close()
	}

	if !foundMatch {
		cmd.Println("No matches found.")
	}

	return nil
}
