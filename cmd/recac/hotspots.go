package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	hotspotsDays  int
	hotspotsLimit int
	hotspotsJSON  bool
)

type Hotspot struct {
	File       string  `json:"file"`
	Churn      int     `json:"churn"`
	Complexity int     `json:"complexity"`
	Score      float64 `json:"score"`
}

var hotspotsCmd = &cobra.Command{
	Use:   "hotspots",
	Short: "Identify code hotspots (High Churn + High Complexity)",
	Long: `Identifies files that are both complex and frequently changed.
These "hotspots" are often high-risk areas for bugs and good candidates for refactoring.

The score is calculated as: Score = Churn * Complexity
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		results, err := runHotspotAnalysis(path, hotspotsDays)
		if err != nil {
			return err
		}

		// Sort by Score
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})

		// Limit
		if hotspotsLimit > 0 && len(results) > hotspotsLimit {
			results = results[:hotspotsLimit]
		}

		if hotspotsJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}

		printHotspotsReport(cmd, results)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(hotspotsCmd)
	hotspotsCmd.Flags().IntVar(&hotspotsDays, "days", 30, "Number of days of git history to analyze")
	hotspotsCmd.Flags().IntVar(&hotspotsLimit, "limit", 10, "Number of top hotspots to show")
	hotspotsCmd.Flags().BoolVar(&hotspotsJSON, "json", false, "Output results as JSON")
}

func runHotspotAnalysis(root string, days int) ([]Hotspot, error) {
	// 1. Get Churn
	churnMap, err := getGitChurn(root, days)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze git churn: %w", err)
	}

	// 2. Get Complexity
	// We use the existing runComplexityAnalysis from complexity.go
	// It returns function-level complexity. We need to aggregate it to file-level.
	funcComplexities, err := runComplexityAnalysis(root)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze complexity: %w", err)
	}

	fileComplexity := make(map[string]int)
	for _, fc := range funcComplexities {
		// Aggregate complexity: Sum of all functions in file
		fileComplexity[fc.File] = fileComplexity[fc.File] + fc.Complexity
	}

	// 3. Combine
	var hotspots []Hotspot
	for file, complexity := range fileComplexity {
		// Clean the path to match git output
		// We make it relative to root and use forward slashes
		cleanPath := file
		if rel, err := filepath.Rel(root, file); err == nil {
			cleanPath = rel
		}
		cleanPath = filepath.ToSlash(cleanPath)

		churn := churnMap[cleanPath]

		// Only consider files with churn > 0 (or at least include them with 0 churn)
		// Usually hotspots have high churn.

		score := float64(churn) * float64(complexity)

		hotspots = append(hotspots, Hotspot{
			File:       cleanPath,
			Churn:      churn,
			Complexity: complexity,
			Score:      score,
		})
	}

	// Also add files that have churn but no complexity (maybe not go files? or failed parsing?)
	// Actually, we probably only care about Go files since complexity is only for Go.
	// So we stick to the loop over fileComplexity.

	return hotspots, nil
}

// getGitChurn executes git log to count file changes
var getGitChurn = func(root string, days int) (map[string]int, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// git log --name-only --relative --format='' --since="2023-01-01"
	cmd := exec.Command("git", "log", "--name-only", "--relative", "--format=", fmt.Sprintf("--since=%s", since))
	cmd.Dir = root

	var out bytes.Buffer
	cmd.Stdout = &out
	// Ignore stderr or handle it? If it's not a git repo, it will fail.

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	churnMap := make(map[string]int)
	scanner := bufio.NewScanner(&out)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		churnMap[line]++
	}

	return churnMap, nil
}

func printHotspotsReport(cmd *cobra.Command, results []Hotspot) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "HOTSPOTS REPORT")
	fmt.Fprintln(w, "---------------")
	fmt.Fprintf(w, "FILE\tSCORE\tCHURN\tCOMPLEXITY\n")

	for _, h := range results {
		fmt.Fprintf(w, "%s\t%.0f\t%d\t%d\n", h.File, h.Score, h.Churn, h.Complexity)
	}
	w.Flush()
}
