package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().String("filter", "", "Filter logs by string match")
	logsCmd.Flags().Bool("all", false, "Stream logs from all running sessions")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs [session-name]",
	Short: "View session logs",
	Long:  `View logs for a specific session or stream logs from all running sessions.`,
	Args: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all && len(args) > 0 {
			return fmt.Errorf("cannot use session name with --all")
		}
		if !all && len(args) != 1 {
			return fmt.Errorf("requires a session name or --all flag")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		follow := cmd.Flags().Lookup("follow").Changed
		filter, _ := cmd.Flags().GetString("filter")

		sm, err := sessionManagerFactory()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		if all {
			streamAllRunningSessions(cmd, sm, follow, filter)
			return
		}

		sessionName := args[0]

		logFile, err := sm.GetSessionLogs(sessionName)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			exit(1)
		}

		file, err := os.Open(logFile)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to open log file: %v\n", err)
			exit(1)
		}
		defer file.Close()

		reader := bufio.NewReader(file)

		// Helper to process line
		processLine := func(line string) {
			if filter == "" || strings.Contains(line, filter) {
				fmt.Fprint(cmd.OutOrStdout(), line)
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
				fmt.Fprintf(cmd.ErrOrStderr(), "Error reading log file: %v\n", err)
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
					fmt.Fprintf(cmd.ErrOrStderr(), "Error streaming logs: %v\n", err)
					break
				}
				processLine(line)
			}
		}
	},
}

func streamAllRunningSessions(cmd *cobra.Command, sm ISessionManager, follow bool, filter string) {
	sessions, err := sm.ListSessions()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to list sessions: %v\n", err)
		exit(1)
	}

	var runningSessions []*runner.SessionState
	for _, s := range sessions {
		if s.Status == "running" {
			runningSessions = append(runningSessions, s)
		}
	}

	if len(runningSessions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No running sessions found.")
		return
	}

	logChan := make(chan string)
	var wg sync.WaitGroup

	for _, session := range runningSessions {
		wg.Add(1)
		go func(s *runner.SessionState) {
			defer wg.Done()
			logFile, err := sm.GetSessionLogs(s.Name)
			if err != nil {
				logChan <- fmt.Sprintf("[%s] Error: %v\n", s.Name, err)
				return
			}

			file, err := os.Open(logFile)
			if err != nil {
				logChan <- fmt.Sprintf("[%s] Error: failed to open log file: %v\n", s.Name, err)
				return
			}
			defer file.Close()

			reader := bufio.NewReader(file)

			// Initial read
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				logChan <- fmt.Sprintf("[%s] %s", s.Name, line)
			}

			if follow {
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							time.Sleep(500 * time.Millisecond)
							continue
						}
						break
					}
					logChan <- fmt.Sprintf("[%s] %s", s.Name, line)
				}
			}
		}(session)
	}

	go func() {
		wg.Wait()
		close(logChan)
	}()

	for line := range logChan {
		if filter == "" || strings.Contains(line, filter) {
			fmt.Fprint(cmd.OutOrStdout(), line)
		}
	}
}
