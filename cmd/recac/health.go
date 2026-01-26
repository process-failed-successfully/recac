package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/security"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	healthJSON      bool
	healthFail      bool
	healthThreshold int
)

var healthCmd = &cobra.Command{
	Use:   "health [path]",
	Short: "Check overall project health",
	Long:  `Aggregates metrics from complexity, security, code duplication, and TODOs to provide a project health report.`,
	RunE:  runHealth,
}

func init() {
	rootCmd.AddCommand(healthCmd)
	healthCmd.Flags().BoolVar(&healthJSON, "json", false, "Output results as JSON")
	healthCmd.Flags().BoolVar(&healthFail, "fail", false, "Exit with error if health score is low or critical issues found")
	healthCmd.Flags().IntVar(&healthThreshold, "threshold", 15, "Complexity threshold for reporting")
}

type HealthReport struct {
	Overview    HealthOverview       `json:"overview"`
	Security    []SecurityResult     `json:"security_issues"`
	Complexity  []FunctionComplexity `json:"high_complexity_functions"`
	Duplication []Duplication        `json:"duplications"`
	Todos       []string             `json:"todos"`
}

type HealthOverview struct {
	TotalFiles         int `json:"total_files"`
	SecurityIssues     int `json:"security_issues_count"`
	HighComplexityFunc int `json:"high_complexity_count"`
	DuplicatedLines    int `json:"duplicated_lines"`
	TodoCount          int `json:"todo_count"`
	HealthScore        int `json:"health_score"`
}

func runHealth(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	report := HealthReport{}

	// 1. File Count (Approx)
	count := 0
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".recac" {
				return filepath.SkipDir
			}
			return nil
		}
		count++
		return nil
	})
	report.Overview.TotalFiles = count

	// 2. Security
	secScanner := security.NewRegexScanner()
	secRes, err := runSecurityScan(root, secScanner)
	if err == nil {
		report.Security = secRes
		report.Overview.SecurityIssues = len(secRes)
	}

	// 3. Complexity
	// We use the flag threshold for identifying "high complexity"
	compRes, err := runComplexityAnalysis(root)
	if err == nil {
		for _, c := range compRes {
			if c.Complexity >= healthThreshold {
				report.Complexity = append(report.Complexity, c)
			}
		}
		// Sort desc
		sort.Slice(report.Complexity, func(i, j int) bool {
			return report.Complexity[i].Complexity > report.Complexity[j].Complexity
		})
		report.Overview.HighComplexityFunc = len(report.Complexity)
	}

	// 4. CPD (Duplication)
	// Use default minLines = 10
	cpdRes, err := runCPD(root, 10, []string{})
	if err == nil {
		report.Duplication = cpdRes
		totalDupLines := 0
		for _, d := range cpdRes {
			totalDupLines += d.LineCount
		}
		report.Overview.DuplicatedLines = totalDupLines
	}

	// 5. TODOs
	todoRes, err := ScanForTodos(root)
	if err == nil {
		// Convert TodoItem objects back to strings for the report
		var todoStrings []string
		for _, t := range todoRes {
			todoStrings = append(todoStrings, t.Raw)
		}
		report.Todos = todoStrings
		report.Overview.TodoCount = len(todoRes)
	}

	// 6. Calculate Score
	// Simple heuristic: Start at 100.
	// -10 per security issue
	// -5 per high complexity function
	// -1 per 10 lines of duplication
	// -1 per 5 TODOs
	score := 100
	score -= (report.Overview.SecurityIssues * 10)
	score -= (report.Overview.HighComplexityFunc * 5)
	score -= (report.Overview.DuplicatedLines / 10)
	score -= (report.Overview.TodoCount / 5)

	if score < 0 {
		score = 0
	}
	report.Overview.HealthScore = score

	// Output
	if healthJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printHealthDashboard(cmd, report)

	if healthFail {
		if report.Overview.SecurityIssues > 0 || score < 50 {
			return fmt.Errorf("health check failed (Score: %d)", score)
		}
	}

	return nil
}

func printHealthDashboard(cmd *cobra.Command, r HealthReport) {
	// Header
	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), "🏥 Project Health Report")
	fmt.Fprintln(cmd.OutOrStdout(), "=======================")

	// Score
	scoreColor := "🟩"
	if r.Overview.HealthScore < 80 {
		scoreColor = "🟨"
	}
	if r.Overview.HealthScore < 50 {
		scoreColor = "🟥"
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Score: %s %d/100\n", scoreColor, r.Overview.HealthScore)
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// Overview Table
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Files Scanned:\t%d\n", r.Overview.TotalFiles)
	fmt.Fprintf(w, "Security Issues:\t%d\n", r.Overview.SecurityIssues)
	fmt.Fprintf(w, "Complex Functions:\t%d (>=%d)\n", r.Overview.HighComplexityFunc, healthThreshold)
	fmt.Fprintf(w, "Duplicated Lines:\t%d\n", r.Overview.DuplicatedLines)
	fmt.Fprintf(w, "TODOs:\t%d\n", r.Overview.TodoCount)
	w.Flush()
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// Details
	if len(r.Security) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "⚠️  Security Issues:")
		for i, s := range r.Security {
			if i >= 5 {
				fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more\n", len(r.Security)-5)
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  - [%s] %s (%s:%d)\n", s.Type, s.Description, s.File, s.Line)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}

	if len(r.Todos) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "📝 Top TODOs:")
		for i, t := range r.Todos {
			if i >= 5 {
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", t)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}

	if len(r.Complexity) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "🧠 Top Complex Functions:")
		for i, c := range r.Complexity {
			if i >= 5 {
				break
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s (Complexity: %d) at %s:%d\n", c.Function, c.Complexity, c.File, c.Line)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}

	if len(r.Duplication) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "👯 Top Duplications:")
		for i, d := range r.Duplication {
			if i >= 3 {
				break
			}
			l1 := d.Locations[0]
			l2 := d.Locations[1]
			fmt.Fprintf(cmd.OutOrStdout(), "  - %d lines between %s:%d and %s:%d\n", d.LineCount, l1.File, l1.StartLine, l2.File, l2.StartLine)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "")
	}
}
