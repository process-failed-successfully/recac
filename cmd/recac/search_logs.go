package main

import (
	"bufio"
	"context"
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
		remote, _ := cmd.Flags().GetBool("remote")

		if useRegexp && caseSensitive {
			return fmt.Errorf("the --regexp and --case-sensitive flags cannot be used together")
		}

		return searchLogs(pattern, cmd, useRegexp, caseSensitive, remote)
	},
}

func init() {
	rootCmd.AddCommand(searchLogsCmd)
	searchLogsCmd.Flags().BoolP("regexp", "r", false, "Enable regular expression matching")
	searchLogsCmd.Flags().BoolP("case-sensitive", "c", false, "Enable case-sensitive matching (cannot be used with --regexp)")
	searchLogsCmd.Flags().Bool("remote", false, "Search in remote Kubernetes pod logs")
}

func searchLogs(pattern string, cmd *cobra.Command, useRegexp, caseSensitive, remote bool) error {
	matchFunc, err := getMatchFunc(pattern, useRegexp, caseSensitive)
	if err != nil {
		return err
	}

	if remote {
		k8sClient, err := k8sClientFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize k8s client: %w", err)
		}
		return doSearchLogsRemote(k8sClient, matchFunc, cmd)
	}

	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}
	return doSearchLogs(sm, matchFunc, cmd)
}

func getMatchFunc(pattern string, useRegexp, caseSensitive bool) (func(string) (bool, error), error) {
	if useRegexp {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regular expression: %w", err)
		}
		return func(line string) (bool, error) {
			return re.MatchString(line), nil
		}, nil
	}

	if caseSensitive {
		return func(line string) (bool, error) {
			return strings.Contains(line, pattern), nil
		}, nil
	}

	lowerPattern := strings.ToLower(pattern)
	return func(line string) (bool, error) {
		return strings.Contains(strings.ToLower(line), lowerPattern), nil
	}, nil
}

func doSearchLogsRemote(client IK8sClient, matchFunc func(string) (bool, error), cmd *cobra.Command) error {
	// List pods
	pods, err := client.ListPods(context.Background(), "app=recac-agent")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	foundMatch := false
	for _, pod := range pods {
		// Fetch logs for each pod
		// We limit to 2000 lines to avoid fetching too much data
		logs, err := client.GetPodLogs(context.Background(), pod.Name, 2000)
		if err != nil {
			cmd.PrintErrln(fmt.Sprintf("warning: failed to get logs for pod %s: %v", pod.Name, err))
			continue
		}

		scanner := bufio.NewScanner(strings.NewReader(logs))
		for scanner.Scan() {
			line := scanner.Text()
			matches, err := matchFunc(line)
			if err != nil {
				return fmt.Errorf("error while matching line in pod %s: %w", pod.Name, err)
			}

			if matches {
				cmd.Println(fmt.Sprintf("[%s] %s", pod.Name, line))
				foundMatch = true
			}
		}
	}

	if !foundMatch {
		cmd.Println("No matches found.")
	}

	return nil
}

func doSearchLogs(sm ISessionManager, matchFunc func(string) (bool, error), cmd *cobra.Command) error {
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
		// We use a closure here to properly close the file in the loop
		func() {
			defer file.Close()
			scanner := bufio.NewScanner(file)

			for scanner.Scan() {
				line := scanner.Text()
				matches, err := matchFunc(line)
				if err != nil {
					// This case should not be reached with the current funcs, but is good practice.
					// We'll log it and continue
					cmd.PrintErrln(fmt.Sprintf("warning: error while matching line in session %s: %v", session.Name, err))
					return
				}

				if matches {
					cmd.Println(fmt.Sprintf("[%s] %s", session.Name, line))
					foundMatch = true
				}
			}
			if err := scanner.Err(); err != nil {
				cmd.PrintErrln(fmt.Sprintf("warning: error reading log file for session %s: %v", session.Name, err))
			}
		}()
	}

	if !foundMatch {
		cmd.Println("No matches found.")
	}

	return nil
}
