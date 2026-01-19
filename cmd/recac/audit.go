package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	auditJSON                bool
	auditFailBelow           int
	auditComplexityThreshold int
	auditCPDMinLines         int
)

type AuditReport struct {
	Score             int                  `json:"score"`
	IssuesFound       int                  `json:"issues_found"`
	ComplexityIssues  []FunctionComplexity `json:"complexity_issues"`
	DuplicationIssues []Duplication        `json:"duplication_issues"`
	TodoIssues        []ScanResult         `json:"todo_issues"`
}

var auditCmd = &cobra.Command{
	Use:   "audit [path]",
	Short: "Perform a comprehensive code health audit",
	Long:  `Aggregates complexity analysis, copy-paste detection, and TODO scanning into a unified health score.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		report, err := runAudit(path)
		if err != nil {
			return err
		}

		if auditJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		displayAuditReport(cmd, report)

		if report.Score < auditFailBelow {
			return fmt.Errorf("audit failed: score %d is below threshold %d", report.Score, auditFailBelow)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.Flags().BoolVar(&auditJSON, "json", false, "Output results as JSON")
	auditCmd.Flags().IntVar(&auditFailBelow, "fail-below", 0, "Fail if score is below this value")
	auditCmd.Flags().IntVar(&auditComplexityThreshold, "threshold-complexity", 10, "Threshold for high complexity")
	auditCmd.Flags().IntVar(&auditCPDMinLines, "threshold-cpd", 10, "Minimum lines for duplication")
}

func runAudit(root string) (*AuditReport, error) {
	// 1. Complexity
	complexityResults, err := runComplexityAnalysis(root)
	if err != nil {
		return nil, fmt.Errorf("complexity analysis failed: %w", err)
	}
	var highComplexity []FunctionComplexity
	for _, res := range complexityResults {
		if res.Complexity >= auditComplexityThreshold {
			highComplexity = append(highComplexity, res)
		}
	}
	// Sort by complexity
	sort.Slice(highComplexity, func(i, j int) bool {
		return highComplexity[i].Complexity > highComplexity[j].Complexity
	})

	// 2. Duplication (CPD)
	cpdResults, err := runCPD(root, auditCPDMinLines, nil)
	if err != nil {
		return nil, fmt.Errorf("cpd analysis failed: %w", err)
	}

	// 3. TODO Scan
	scanResults, err := runScan(root)
	if err != nil {
		return nil, fmt.Errorf("scan failed: %w", err)
	}

	// 4. Calculate Score
	// Base score: 100
	// Penalties:
	// - 5 points per high complexity function (capped at 30)
	// - 10 points per duplication block (capped at 40)
	// - 1 point per TODO (capped at 20)
	// Min score: 0

	score := 100

	compPenalty := len(highComplexity) * 5
	if compPenalty > 30 {
		compPenalty = 30
	}

	cpdPenalty := len(cpdResults) * 10
	if cpdPenalty > 40 {
		cpdPenalty = 40
	}

	todoPenalty := len(scanResults) * 1
	if todoPenalty > 20 {
		todoPenalty = 20
	}

	score = score - compPenalty - cpdPenalty - todoPenalty
	if score < 0 {
		score = 0
	}

	return &AuditReport{
		Score:             score,
		IssuesFound:       len(highComplexity) + len(cpdResults) + len(scanResults),
		ComplexityIssues:  highComplexity,
		DuplicationIssues: cpdResults,
		TodoIssues:        scanResults,
	}, nil
}

func displayAuditReport(cmd *cobra.Command, report *AuditReport) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "AUDIT REPORT\n")
	fmt.Fprintf(w, "------------\n")
	fmt.Fprintf(w, "Health Score:\t%d / 100\n", report.Score)
	fmt.Fprintf(w, "Total Issues:\t%d\n", report.IssuesFound)
	fmt.Fprintln(w, "")

	if len(report.ComplexityIssues) > 0 {
		fmt.Fprintln(w, "High Complexity Functions (>"+fmt.Sprintf("%d", auditComplexityThreshold)+"):")
		for _, c := range report.ComplexityIssues {
			fmt.Fprintf(w, "  - %s (%d) in %s:%d\n", c.Function, c.Complexity, c.File, c.Line)
		}
		fmt.Fprintln(w, "")
	}

	if len(report.DuplicationIssues) > 0 {
		fmt.Fprintln(w, "Duplicated Code Blocks (>"+fmt.Sprintf("%d", auditCPDMinLines)+" lines):")
		for _, d := range report.DuplicationIssues {
			l1 := d.Locations[0]
			fmt.Fprintf(w, "  - %d lines in %s:%d-%d ...\n", d.LineCount, l1.File, l1.StartLine, l1.EndLine)
		}
		fmt.Fprintln(w, "")
	}

	if len(report.TodoIssues) > 0 {
		fmt.Fprintln(w, "Technical Debt Markers:")
		// Show top 5 only if too many
		limit := 5
		for i, t := range report.TodoIssues {
			if i >= limit {
				fmt.Fprintf(w, "  ... and %d more\n", len(report.TodoIssues)-limit)
				break
			}
			fmt.Fprintf(w, "  - %s: %s (%s:%d)\n", t.Type, t.Message, t.File, t.Line)
		}
	}

	w.Flush()
}
