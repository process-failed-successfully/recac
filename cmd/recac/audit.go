package main

import (
	"encoding/json"
	"fmt"
	"math"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	auditPath       string
	auditMinScore   int
	auditFail       bool
	auditJson       bool
	auditCompThresh int
)

type AuditResult struct {
	Score       int              `json:"score"`
	Complexity  ComplexityStats  `json:"complexity"`
	Duplication DuplicationStats `json:"duplication"`
	Todos       TodoStats        `json:"todos"`
	Passed      bool             `json:"passed"`
}

type ComplexityStats struct {
	Average       float64              `json:"average"`
	Max           int                  `json:"max"`
	HighRiskFuncs []FunctionComplexity `json:"high_risk_funcs"`
}

type DuplicationStats struct {
	Blocks int `json:"blocks"`
	Lines  int `json:"lines"`
}

type TodoStats struct {
	Count int `json:"count"`
}

var auditCmd = &cobra.Command{
	Use:   "audit [path]",
	Short: "Perform a comprehensive code health audit",
	Long: `Runs multiple static analysis tools (Complexity, CPD, TODOs) to generate a
health score for the codebase.

Scoring (out of 100):
- High Complexity Functions: Penalty
- Code Duplication: Penalty
- TODOs: Small Penalty
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			auditPath = args[0]
		}
		if auditPath == "" {
			auditPath = "."
		}

		result, err := runAudit(auditPath)
		if err != nil {
			return err
		}

		if auditJson {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return err
			}
		} else {
			printAuditReport(cmd, result)
		}

		if auditFail && !result.Passed {
			return fmt.Errorf("audit failed: score %d < %d", result.Score, auditMinScore)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().StringVarP(&auditPath, "path", "p", ".", "Path to analyze")
	auditCmd.Flags().IntVar(&auditMinScore, "min-score", 80, "Minimum passing score")
	auditCmd.Flags().BoolVar(&auditFail, "fail", false, "Exit with error if score below minimum")
	auditCmd.Flags().BoolVar(&auditJson, "json", false, "Output results as JSON")
	auditCmd.Flags().IntVar(&auditCompThresh, "complexity-threshold", 15, "Threshold for high complexity functions")
}

func runAudit(root string) (*AuditResult, error) {
	res := &AuditResult{
		Score: 100,
	}

	// 1. Complexity
	// Reuse runComplexityAnalysis from complexity.go
	compResults, err := runComplexityAnalysis(root)
	if err != nil {
		return nil, fmt.Errorf("complexity analysis failed: %w", err)
	}

	var totalComp int
	var maxComp int
	var highRisk []FunctionComplexity

	for _, c := range compResults {
		totalComp += c.Complexity
		if c.Complexity > maxComp {
			maxComp = c.Complexity
		}
		if c.Complexity >= auditCompThresh {
			highRisk = append(highRisk, c)
		}
	}

	if len(compResults) > 0 {
		res.Complexity.Average = float64(totalComp) / float64(len(compResults))
	}
	res.Complexity.Max = maxComp
	res.Complexity.HighRiskFuncs = highRisk

	// 2. CPD (Duplication)
	// Reuse runCPD from cpd.go
	// Default minLines=10, ignore=[]
	cpdResults, err := runCPD(root, 10, nil)
	if err != nil {
		return nil, fmt.Errorf("cpd analysis failed: %w", err)
	}

	var dupLines int
	for _, d := range cpdResults {
		dupLines += d.LineCount
	}
	res.Duplication.Blocks = len(cpdResults)
	res.Duplication.Lines = dupLines

	// 3. TODOs
	// Reuse scanTodos from todo_scan.go
	todoResults, err := scanTodos(root)
	if err != nil {
		return nil, fmt.Errorf("todo scan failed: %w", err)
	}
	res.Todos.Count = len(todoResults)

	// 4. Calculate Score
	score := 100.0

	// Deduct for complexity
	// -2 for every function over threshold
	score -= float64(len(highRisk)) * 2
	// -5 if max complexity is really high (> 2 * threshold)
	if maxComp > auditCompThresh*2 {
		score -= 5
	}

	// Deduct for duplication
	// -2 for every duplicated block
	score -= float64(len(cpdResults)) * 2

	// Deduct for TODOs
	// -0.1 per TODO (capped at 10 points)
	todoPenalty := float64(len(todoResults)) * 0.1
	if todoPenalty > 10 {
		todoPenalty = 10
	}
	score -= todoPenalty

	if score < 0 {
		score = 0
	}
	res.Score = int(math.Round(score))

	if res.Score >= auditMinScore {
		res.Passed = true
	}

	return res, nil
}

func printAuditReport(cmd *cobra.Command, res *AuditResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "AUDIT REPORT\t")
	fmt.Fprintln(w, "------------\t")

	scoreIcon := "🟢"
	if res.Score < 80 {
		scoreIcon = "🟡"
	}
	if res.Score < 60 {
		scoreIcon = "🔴"
	}

	fmt.Fprintf(w, "Score\t%s %d / 100\n", scoreIcon, res.Score)
	fmt.Fprintf(w, "Status\t%s\n", func() string {
		if res.Passed {
			return "PASS"
		}
		return "FAIL"
	}())
	fmt.Fprintln(w, "\t")

	fmt.Fprintln(w, "COMPLEXITY\t")
	fmt.Fprintf(w, "  Average\t%.2f\n", res.Complexity.Average)
	fmt.Fprintf(w, "  Max\t%d\n", res.Complexity.Max)
	fmt.Fprintf(w, "  High Risk (>%d)\t%d\n", auditCompThresh, len(res.Complexity.HighRiskFuncs))
	fmt.Fprintln(w, "\t")

	fmt.Fprintln(w, "DUPLICATION\t")
	fmt.Fprintf(w, "  Blocks\t%d\n", res.Duplication.Blocks)
	fmt.Fprintf(w, "  Lines\t%d\n", res.Duplication.Lines)
	fmt.Fprintln(w, "\t")

	fmt.Fprintln(w, "MAINTENANCE\t")
	fmt.Fprintf(w, "  TODOs\t%d\n", res.Todos.Count)
	fmt.Fprintln(w, "\t")

	w.Flush()

	if len(res.Complexity.HighRiskFuncs) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nTop 5 High Complexity Functions:")
		limit := 5
		if len(res.Complexity.HighRiskFuncs) < limit {
			limit = len(res.Complexity.HighRiskFuncs)
		}
		for i := 0; i < limit; i++ {
			fn := res.Complexity.HighRiskFuncs[i]
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%s:%d): %d\n", fn.Function, fn.File, fn.Line, fn.Complexity)
		}
	}
}
