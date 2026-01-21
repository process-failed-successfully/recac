package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var evolutionCmd = &cobra.Command{
	Use:   "evolution [git-range]",
	Short: "Track codebase evolution metrics over time",
	Long: `Analyzes the codebase evolution by iterating through git commits and calculating metrics.
Default range is HEAD~10..HEAD.

Metrics:
- Complexity: Total cyclomatic complexity of all Go functions.
- TODOs: Total number of TODO/FIXME items.
- LOC: Total Lines of Code (Go files).
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runEvolution,
}

func init() {
	rootCmd.AddCommand(evolutionCmd)
}

type CommitMetric struct {
	Hash       string
	Subject    string
	Complexity int
	Todos      int
	LOC        int
}

func runEvolution(cmd *cobra.Command, args []string) error {
	gitRange := "HEAD~10..HEAD"
	if len(args) > 0 {
		gitRange = args[0]
	}

	// 1. Get Commits
	commits, err := getCommits(gitRange)
	if err != nil {
		return fmt.Errorf("failed to get commits for range '%s': %w (ensure the range is valid and history exists)", gitRange, err)
	}
	if len(commits) == 0 {
		fmt.Println("No commits found in range.")
		return nil
	}

	fmt.Printf("Analyzing %d commits...\n", len(commits))

	// 2. Setup Temp Directory
	// We create a base temp dir to hold our worktree
	tmpBase, err := os.MkdirTemp("", "recac-evolution")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpBase) // Clean up everything at the end

	worktreeDir := filepath.Join(tmpBase, "worktree")

	// Helper to cleanup worktree
	cleanupWorktree := func() {
		// Prune the worktree info from main repo
		exec.Command("git", "worktree", "remove", worktreeDir, "--force").Run()
		// Ensure dir is gone
		os.RemoveAll(worktreeDir)
		// Prune metadata
		exec.Command("git", "worktree", "prune").Run()
	}
	defer cleanupWorktree()

	var metrics []CommitMetric

	for i, commit := range commits {
		fmt.Printf("[%d/%d] Analyzing %s...\n", i+1, len(commits), commit[:7])

		cleanupWorktree()

		// Create worktree
		// git worktree add --detach <path> <commit>
		out, err := exec.Command("git", "worktree", "add", "--detach", worktreeDir, commit).CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to create worktree for %s: %s", commit, string(out))
		}

		// Analyze
		m := CommitMetric{Hash: commit}

		// Get Subject
		subj, _ := exec.Command("git", "log", "-1", "--format=%s", commit).Output()
		m.Subject = strings.TrimSpace(string(subj))
		if len(m.Subject) > 30 {
			m.Subject = m.Subject[:27] + "..."
		}

		// Complexity
		compRes, err := runComplexityAnalysis(worktreeDir)
		if err == nil {
			totalComp := 0
			for _, c := range compRes {
				totalComp += c.Complexity
			}
			m.Complexity = totalComp
		}

		// TODOs
		todos, err := scanTodos(worktreeDir)
		if err == nil {
			m.Todos = len(todos)
		}

		// LOC
		m.LOC, _ = countLOC(worktreeDir)

		metrics = append(metrics, m)
	}

	// 3. Display
	renderChart(cmd, metrics)

	return nil
}

func getCommits(gitRange string) ([]string, error) {
	cmd := exec.Command("git", "rev-list", "--reverse", gitRange)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var commits []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			commits = append(commits, strings.TrimSpace(l))
		}
	}
	return commits, nil
}

func countLOC(root string) (int, error) {
	loc := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if info.IsDir() {
			// Skip hidden dirs, but allow the root itself
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." && path != root {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			// Simple line count
			f, err := os.Open(path)
			if err != nil { return nil }
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				loc++
			}
		}
		return nil
	})
	return loc, err
}

func renderChart(cmd *cobra.Command, metrics []CommitMetric) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\nCODEBASE EVOLUTION")
	fmt.Fprintln(w, "Commit\tSubject\tLOC\tComplex\tTODOs\tTrend (Complexity)")

	maxComplex := 0
	for _, m := range metrics {
		if m.Complexity > maxComplex {
			maxComplex = m.Complexity
		}
	}

	if maxComplex == 0 { maxComplex = 1 }

	for _, m := range metrics {
		barLen := int(math.Round(float64(m.Complexity) / float64(maxComplex) * 20))
		bar := strings.Repeat("#", barLen)
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%d\t%s\n", m.Hash[:7], m.Subject, m.LOC, m.Complexity, m.Todos, bar)
	}
	w.Flush()
}
