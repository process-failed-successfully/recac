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
	hotspotsChart string
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

		if hotspotsChart == "quadrant" {
			fmt.Fprintln(cmd.OutOrStdout(), generateMermaidQuadrant(results))
			return nil
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
	hotspotsCmd.Flags().StringVar(&hotspotsChart, "chart", "", "Output format: 'quadrant' for Mermaid chart")
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

func generateMermaidQuadrant(hotspots []Hotspot) string {
	if len(hotspots) == 0 {
		return "graph TD\n    NoData[No Hotspots Found]"
	}

	maxChurn := 0
	maxComplexity := 0

	for _, h := range hotspots {
		if h.Churn > maxChurn {
			maxChurn = h.Churn
		}
		if h.Complexity > maxComplexity {
			maxComplexity = h.Complexity
		}
	}

	// Avoid division by zero
	if maxChurn == 0 {
		maxChurn = 1
	}
	if maxComplexity == 0 {
		maxComplexity = 1
	}

	var sb strings.Builder
	sb.WriteString("quadrantChart\n")
	sb.WriteString("    title Hotspots Analysis (Churn vs Complexity)\n")
	sb.WriteString("    x-axis Low Churn --> High Churn\n")
	sb.WriteString("    y-axis Low Complexity --> High Complexity\n")
	sb.WriteString("    quadrant-1 Refactor Candidates\n")
	sb.WriteString("    quadrant-2 Complicated & Stable\n")
	sb.WriteString("    quadrant-3 Healthy\n")
	sb.WriteString("    quadrant-4 Rapid Prototyping\n")

	for _, h := range hotspots {
		// Normalize to 0.05 - 0.95 to avoid edge clipping
		x := 0.05 + (float64(h.Churn)/float64(maxChurn))*0.9
		y := 0.05 + (float64(h.Complexity)/float64(maxComplexity))*0.9

		// Truncate long filenames
		label := h.File
		if len(label) > 30 {
			label = "..." + label[len(label)-27:]
		}
		// Escape quotes in label if any (though filenames rarely have quotes)
		label = strings.ReplaceAll(label, "\"", "'")

		sb.WriteString(fmt.Sprintf("    \"%s\": [%.2f, %.2f]\n", label, x, y))
	}

	return sb.String()
}
