package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var (
	evolutionDays int
	evolutionJSON bool
)

type EvolutionMetric struct {
	Date       string `json:"date"`
	Commit     string `json:"commit"`
	LOC        int    `json:"loc"`
	Complexity int    `json:"complexity"`
	TODOs      int    `json:"todos"`
}

var evolutionCmd = &cobra.Command{
	Use:   "evolution",
	Short: "Analyze codebase evolution (LOC, Complexity, TODOs)",
	Long: `Analyzes the git history to track how code metrics have evolved over time.
It checks out historical commits (using git worktree) and calculates:
- Lines of Code (LOC)
- Cyclomatic Complexity
- Number of TODOs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		metrics, err := runEvolutionAnalysis(cmd, root, evolutionDays)
		if err != nil {
			return err
		}

		if evolutionJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(metrics)
		}

		printEvolutionReport(cmd, metrics)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(evolutionCmd)
	evolutionCmd.Flags().IntVar(&evolutionDays, "days", 30, "Number of days of history to analyze")
	evolutionCmd.Flags().BoolVar(&evolutionJSON, "json", false, "Output results as JSON")
}

func runEvolutionAnalysis(cmd *cobra.Command, root string, days int) ([]EvolutionMetric, error) {
	// 1. Get commits
	commits, err := getCommits(root, days)
	if err != nil {
		return nil, err
	}

	if len(commits) == 0 {
		return nil, fmt.Errorf("no commits found in the last %d days", days)
	}

	fmt.Fprintf(cmd.OutOrStderr(), "Found %d commits. Analyzing...\n", len(commits))

	// 2. Setup worktree
	tempDir, err := os.MkdirTemp("", "recac-evolution-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Prune worktrees on exit just in case
	defer func() {
		execCommand("git", "worktree", "prune").Run()
	}()

	var metrics []EvolutionMetric

	// 3. Iterate commits (sample if too many?)
	// Let's take at most 10 sample points to avoid taking forever
	step := 1
	if len(commits) > 10 {
		step = len(commits) / 10
	}

	// Iterate backwards (oldest to newest)
	for i := len(commits) - 1; i >= 0; i -= step {
		c := commits[i]

		fmt.Fprintf(cmd.OutOrStderr(), "Analyzing %s (%s)...\n", c.Hash[:7], c.Date)

		// Checkout to temp dir using worktree
		// git worktree add --detach <path> <commit>
		// We need to remove the worktree before adding a new one or reuse it.
		// Reusing/updating a worktree is tricky. Easiest is to add, analyze, remove.

		// Clean up previous worktree if any (though we use unique temp dir per run, we reuse it?)
		// No, let's create a sub-folder for the worktree
		wtDir := filepath.Join(tempDir, "wt")

		// Ensure it doesn't exist
		os.RemoveAll(wtDir)

		// Run git worktree add
		// We must run this from the repo root
		gitCmd := execCommand("git", "worktree", "add", "--detach", wtDir, c.Hash)
		gitCmd.Dir = root
		if out, err := gitCmd.CombinedOutput(); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to checkout %s: %v\nOutput: %s\n", c.Hash, err, out)
			continue
		}

		// Analyze
		m, err := analyzeSnapshot(wtDir)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Warning: failed to analyze %s: %v\n", c.Hash, err)
			// Cleanup worktree
			execCommand("git", "worktree", "remove", "--force", wtDir).Run()
			continue
		}
		m.Date = c.Date
		m.Commit = c.Hash
		metrics = append(metrics, m)

		// Cleanup worktree
		cmdCleanup := execCommand("git", "worktree", "remove", "--force", wtDir)
		cmdCleanup.Dir = root
		cmdCleanup.Run()
	}

	return metrics, nil
}

type commitInfo struct {
	Hash string
	Date string
}

func getCommits(root string, days int) ([]commitInfo, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// git log --since="2023-01-01" --format="%H %as" (hash YYYY-MM-DD)
	cmd := execCommand("git", "log", fmt.Sprintf("--since=%s", since), "--format=%H %as")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []commitInfo
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), " ")
		if len(parts) >= 2 {
			commits = append(commits, commitInfo{
				Hash: parts[0],
				Date: parts[1],
			})
		}
	}
	return commits, nil
}

func analyzeSnapshot(root string) (EvolutionMetric, error) {
	var m EvolutionMetric

	// 1. LOC
	loc, err := countLOC(root)
	if err != nil {
		return m, err
	}
	m.LOC = loc

	// 2. Complexity
	// runComplexityAnalysis is from complexity.go (package main)
	complexities, err := runComplexityAnalysis(root)
	if err != nil {
		// If parsing fails (e.g. invalid code in history), just return 0
		// But let's log it? No, just continue.
	}
	totalComp := 0
	for _, c := range complexities {
		totalComp += c.Complexity
	}
	m.Complexity = totalComp

	// 3. TODOs
	// ScanForTodos is from todo_scan.go (package main)
	todos, err := ScanForTodos(root)
	if err != nil {
		// ignore
	}
	m.TODOs = len(todos)

	return m, nil
}

func countLOC(root string) (int, error) {
	count := 0
	ignoreMap := DefaultIgnoreMap()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignoreMap[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Count lines
		lines, err := utils.ReadLines(path)
		if err != nil {
			return nil
		}
		// Basic LOC: skip empty lines and comments?
		// For consistency, let's just count non-empty lines
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				count++
			}
		}
		return nil
	})
	return count, err
}

func printEvolutionReport(cmd *cobra.Command, metrics []EvolutionMetric) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "\nEVOLUTION REPORT")
	fmt.Fprintln(w, "----------------")
	fmt.Fprintln(w, "DATE\tCOMMIT\tLOC\tCOMPLEXITY\tTODOs")

	for _, m := range metrics {
		shortHash := m.Commit
		if len(shortHash) > 7 {
			shortHash = shortHash[:7]
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\n", m.Date, shortHash, m.LOC, m.Complexity, m.TODOs)
	}
	w.Flush()

	// Simple ASCII Chart if enough data
	if len(metrics) > 1 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nTRENDS:")
		printTrend(cmd, "LOC", metrics, func(m EvolutionMetric) int { return m.LOC })
		printTrend(cmd, "Complexity", metrics, func(m EvolutionMetric) int { return m.Complexity })
		printTrend(cmd, "TODOs", metrics, func(m EvolutionMetric) int { return m.TODOs })
	}
}

func printTrend(cmd *cobra.Command, label string, metrics []EvolutionMetric, extractor func(m EvolutionMetric) int) {
	fmt.Fprintf(cmd.OutOrStdout(), "[%s] ", label)

	vals := make([]int, len(metrics))
	minVal, maxVal := 999999999, 0
	for i, m := range metrics {
		v := extractor(m)
		vals[i] = v
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Flat (0)")
		return
	}

	// Sparkline-ish
	start := vals[0]
	end := vals[len(vals)-1]
	diff := end - start

	sign := "+"
	if diff < 0 {
		sign = ""
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%d -> %d (%s%d)\n", start, end, sign, diff)
}
