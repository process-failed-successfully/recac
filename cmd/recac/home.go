package main

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"recac/internal/tui"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var homeCmd = &cobra.Command{
	Use:   "home",
	Short: "Show the developer dashboard",
	Long:  `Displays a dashboard with git status, recent sessions, and tasks.`,
	RunE:  runHome,
}

func init() {
	rootCmd.AddCommand(homeCmd)
}

func runHome(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Git Status
	gitStatus := tui.GitStatus{
		Branch: "Unknown",
	}

	// Use factory
	gClient := gitClientFactory()

	// Check if repo
	if gClient.RepoExists(cwd) {
		branch, err := gClient.CurrentBranch(cwd)
		if err == nil {
			gitStatus.Branch = branch
		}

		statusOut, err := gClient.Run(cwd, "status", "--porcelain")
		if err == nil && len(strings.TrimSpace(statusOut)) > 0 {
			gitStatus.Dirty = true
		}

		// Last Commit
		sha, err := gClient.CurrentCommitSHA(cwd)
		if err == nil {
			if len(sha) > 7 {
				gitStatus.LastCommitHash = sha[:7]
			} else {
				gitStatus.LastCommitHash = sha
			}

			msg, err := gClient.Run(cwd, "log", "-1", "--pretty=%s")
			if err == nil {
				gitStatus.LastCommitMsg = strings.TrimSpace(msg)
			}
		}
	}

	// 2. Sessions
	sm, err := sessionManagerFactory()
	var sessions []tui.RecentSession
	if err == nil {
		list, err := sm.ListSessions()
		if err == nil {
			// Sort by start time desc
			sort.Slice(list, func(i, j int) bool {
				return list[i].StartTime.After(list[j].StartTime)
			})

			count := 0
			for _, s := range list {
				if count >= 3 {
					break
				}
				sessions = append(sessions, tui.RecentSession{
					Name:    s.Name,
					Status:  s.Status,
					Time:    s.StartTime,
					Elapsed: time.Since(s.StartTime), // Approximate
				})
				count++
			}
		}
	}

	// 3. Todos
	// ScanForTodos is in todo_scan.go (package main)
	todos, err := ScanForTodos(cwd)
	todoSummary := tui.TodoSummary{}
	if err == nil {
		todoSummary.Count = len(todos)
		for _, t := range todos {
			// Keywords are stored in TodoItem (defined in todo_scan.go)
			kw := strings.ToUpper(t.Keyword)
			if kw == "FIXME" || kw == "BUG" || kw == "CRITICAL" {
				todoSummary.Critical++
			}
		}
	}

	// 4. System Info
	sysInfo := tui.SystemInfo{
		OS: runtime.GOOS,
		// Memory/CPU require generic lib or specific commands. Skip for now.
	}

	// Start TUI
	m := tui.NewHomeModel(gitStatus, todoSummary, sessions, sysInfo)
	return runTUIFunc(m)
}

// runTUIFunc is a variable to allow mocking the TUI execution in tests
var runTUIFunc = func(m tui.HomeModel) error {
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running home dashboard: %w", err)
	}
	return nil
}
