package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display a summary of the current repository state",
	Long:  `Provides a detailed summary of the current repository state including git branch, changes, file count, line count, TODO count, and RECAC sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		gitClient := gitClientFactory()

		isRepo := gitClient.RepoExists(cwd)
		branch := "N/A"
		commitCount := "0"
		stagedChanges := 0
		unstagedChanges := 0

		if isRepo {
			b, err := gitClient.CurrentBranch(cwd)
			if err == nil && b != "" {
				branch = b
			}

			// Commit count
			out, err := gitClient.Run(cwd, "rev-list", "--count", "HEAD")
			if err == nil {
				commitCount = strings.TrimSpace(out)
			}

			// Staged/Unstaged
			statusOut, err := gitClient.Run(cwd, "status", "--porcelain")
			if err == nil {
				lines := strings.Split(statusOut, "\n")
				for _, line := range lines {
					if len(line) < 2 {
						continue
					}
					x := line[0] // index status
					y := line[1] // working tree status

					if x != ' ' && x != '?' {
						stagedChanges++
					}
					if y != ' ' && y != '?' {
						unstagedChanges++
					}
					// If untracked: x='?', y='?'
					if x == '?' && y == '?' {
						unstagedChanges++
					}
				}
			}
		}

		// File / Line count
		ignoreMap := DefaultIgnoreMap()
		fileCount := 0
		lineCount := 0

		_ = filepath.WalkDir(cwd, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if d.IsDir() {
				if ignoreMap[d.Name()] {
					return filepath.SkipDir
				}
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
					return filepath.SkipDir
				}
				return nil
			}

			if ignoreMap[d.Name()] {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}

			// Skip binary files (by extension)
			ext := strings.ToLower(filepath.Ext(path))
			if utils.IsBinaryExt(ext) {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()

			fileCount++

			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				lineCount++
			}
			return nil
		})

		// TODOs
		todos, err := ScanForTodos(cwd)
		todoCount := 0
		if err == nil {
			todoCount = len(todos)
		}

		// Sessions & Cost
		sessionCount := 0
		var totalCost float64
		if sm, err := sessionManagerFactory(); err == nil {
			if sessions, err := sm.ListSessions(); err == nil {
				sessionCount = len(sessions)
				for _, s := range sessions {
					if s.AgentStateFile != "" {
						if agentState, err := loadAgentState(s.AgentStateFile); err == nil {
							totalCost += agent.CalculateCost(agentState.Model, agentState.TokenUsage)
						}
					}
				}
			}
		}

		// Output
		fmt.Fprintln(cmd.OutOrStdout(), "Repository Info:")
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintf(w, "  Git Branch:\t%s\n", branch)
		if isRepo {
			fmt.Fprintf(w, "  Commits:\t%s\n", commitCount)
			fmt.Fprintf(w, "  Staged Changes:\t%d\n", stagedChanges)
			fmt.Fprintf(w, "  Unstaged Changes:\t%d\n", unstagedChanges)
		}
		fmt.Fprintf(w, "  Total Files:\t%d\n", fileCount)
		fmt.Fprintf(w, "  Total Lines:\t%d\n", lineCount)
		fmt.Fprintf(w, "  TODO Count:\t%d\n", todoCount)
		fmt.Fprintf(w, "  Total Sessions:\t%d\n", sessionCount)
		fmt.Fprintf(w, "  Estimated AI Cost:\t$%.2f\n", totalCost)
		w.Flush()

		return nil
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
