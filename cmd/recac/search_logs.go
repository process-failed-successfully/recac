package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var searchLogsCmd = &cobra.Command{
	Use:   "search-logs [pattern]",
	Short: "Search for a pattern in all session logs",
	Long: `Scans through all session log files and prints lines that match the provided pattern.
By default, the search is case-insensitive. Use flags to enable case-sensitive or regex matching.
Each matching line is prefixed with the session name for context.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		pattern := args[0]

		useRegexp, _ := cmd.Flags().GetBool("regexp")
		caseSensitive, _ := cmd.Flags().GetBool("case-sensitive")

		if useRegexp && caseSensitive {
			return fmt.Errorf("the --regexp and --case-sensitive flags cannot be used together")
		}

		return searchLogs(pattern, cmd, useRegexp, caseSensitive)
	},
}

func init() {
	rootCmd.AddCommand(searchLogsCmd)
	searchLogsCmd.Flags().BoolP("regexp", "r", false, "Enable regular expression matching")
	searchLogsCmd.Flags().BoolP("case-sensitive", "c", false, "Enable case-sensitive matching (cannot be used with --regexp)")
}

func searchLogs(pattern string, cmd *cobra.Command, useRegexp, caseSensitive bool) error {
	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	return doSearchLogs(sm, pattern, cmd, useRegexp, caseSensitive)
}

func doSearchLogs(sm ISessionManager, pattern string, cmd *cobra.Command, useRegexp, caseSensitive bool) error {
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
		defer file.Close()

		scanner := bufio.NewScanner(file)

		var matchFunc func(string) (bool, error)
		if useRegexp {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return fmt.Errorf("invalid regular expression: %w", err)
			}
			matchFunc = func(line string) (bool, error) {
				return re.MatchString(line), nil
			}
		} else if caseSensitive {
			matchFunc = func(line string) (bool, error) {
				return strings.Contains(line, pattern), nil
			}
		} else {
			lowerPattern := strings.ToLower(pattern)
			matchFunc = func(line string) (bool, error) {
				return strings.Contains(strings.ToLower(line), lowerPattern), nil
			}
		}

		for scanner.Scan() {
			line := scanner.Text()
			matches, err := matchFunc(line)
			if err != nil {
				// This case should not be reached with the current funcs, but is good practice.
				return fmt.Errorf("error while matching line in session %s: %w", session.Name, err)
			}

			if matches {
				cmd.Println(fmt.Sprintf("[%s] %s", session.Name, line))
				foundMatch = true
			}
		}
		if err := scanner.Err(); err != nil {
			cmd.PrintErrln(fmt.Sprintf("warning: error reading log file for session %s: %v", session.Name, err))
		}
	}

	if !foundMatch {
		cmd.Println("No matches found.")
	}

	return nil
}
