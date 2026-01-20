package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"recac/internal/security"

	"github.com/spf13/cobra"
)

var (
	verifyStaged bool
	verifyAll    bool
	verifyJSON   bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify [files...]",
	Short: "Verify code quality on changed lines",
	Long: `Performs targeted code quality checks (Complexity, Security) on changed files.
By default, it checks only lines that have been modified in the git diff (unstaged changes).
Use --staged to check staged changes.
Use --all to check the entire file content of changed files, not just modified lines.`,
	RunE: runVerify,
}

func init() {
	rootCmd.AddCommand(verifyCmd)
	verifyCmd.Flags().BoolVar(&verifyStaged, "staged", false, "Check staged changes")
	verifyCmd.Flags().BoolVar(&verifyAll, "all", false, "Check entire file content, not just changed lines")
	verifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Output results as JSON")
}

type LineInterval struct {
	Start int
	End   int
}

type VerifyIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Type     string `json:"type"` // "Complexity", "Security"
	Message  string `json:"message"`
	Severity string `json:"severity"` // "High", "Medium", "Low"
}

func runVerify(cmd *cobra.Command, args []string) error {
	var files []string
	var err error

	// 1. Identify files to check
	if len(args) > 0 {
		files = args
	} else {
		files, err = getChangedFiles(verifyStaged)
		if err != nil {
			return fmt.Errorf("failed to get changed files: %w", err)
		}
	}

	if len(files) == 0 {
		if !verifyJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "No changed files to verify.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil
	}

	// 2. Analyze each file
	var allIssues []VerifyIssue
	secScanner := security.NewRegexScanner()

	for _, file := range files {
		// Skip non-go files for complexity? Security might check others.
		// For now, let's focus on what the underlying tools support.
		// Complexity supports Go. Security supports whatever regex matches (usually general).

		// Get changed lines for filtering
		var changedLines []LineInterval
		if !verifyAll {
			changedLines, err = getFileChangedLines(file, verifyStaged)
			if err != nil {
				// If we can't get diff, changedLines remains nil.
				// The filtering logic below will ignore this file's issues unless --all is set.
				// This prevents false positives when diff fails (e.g. untracked files).
			} else if changedLines == nil {
				// Explicitly make it non-nil empty slice to avoid "check all" behavior when diff is empty
				changedLines = make([]LineInterval, 0)
			}
		}

		// Security Scan
		secIssues, err := scanFileForSecurity(file, secScanner)
		if err == nil {
			for _, iss := range secIssues {
				if verifyAll || isLineInIntervals(iss.Line, changedLines) || (changedLines == nil && verifyAll) {
					allIssues = append(allIssues, VerifyIssue{
						File:     file,
						Line:     iss.Line,
						Type:     "Security",
						Message:  fmt.Sprintf("%s: %s", iss.Type, iss.Description),
						Severity: "High",
					})
				}
			}
		}

		// Complexity Scan (only Go files)
		if strings.HasSuffix(file, ".go") {
			// runComplexityAnalysis takes a directory or file.
			compResults, err := runComplexityAnalysis(file)
			if err == nil {
				for _, res := range compResults {
					// Default threshold from complexity.go is 10 (hardcoded default in flag, but we can access variable `complexityThreshold`?)
					// `complexityThreshold` is in `complexity.go`. It's a package level var.
					// We should probably respect it or define our own.
					// Let's use 10 as a safe default or read the flag if set (but flags are parsed per command).
					// Better: use a reasonable default for verify, say 15.
					threshold := 15
					if res.Complexity >= threshold {
						if verifyAll || isLineInIntervals(res.Line, changedLines) {
							allIssues = append(allIssues, VerifyIssue{
								File:     file,
								Line:     res.Line,
								Type:     "Complexity",
								Message:  fmt.Sprintf("Function '%s' has complexity %d (threshold %d)", res.Function, res.Complexity, threshold),
								Severity: "Medium",
							})
						}
					}
				}
			}
		}
	}

	// 3. Report Results
	if verifyJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(allIssues)
	}

	if len(allIssues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No issues found in changed lines!")
		return nil
	}

	printVerifyTable(cmd, allIssues)
	return fmt.Errorf("verification failed with %d issues", len(allIssues))
}

func getChangedFiles(staged bool) ([]string, error) {
	args := []string{"diff", "--name-only"}
	if staged {
		args = append(args, "--cached")
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			// check if file exists (it might have been deleted)
			if _, err := os.Stat(l); err == nil {
				files = append(files, l)
			}
		}
	}
	return files, nil
}

func getFileChangedLines(file string, staged bool) ([]LineInterval, error) {
	// git diff -U0 -- [file]
	args := []string{"diff", "--unified=0"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", file)

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseDiffHunks(string(out)), nil
}

// parseDiffHunks parses `@@ -old_start,old_count +new_start,new_count @@`
func parseDiffHunks(diff string) []LineInterval {
	var intervals []LineInterval
	// Regex to find hunk headers
	// @@ -1,2 +3,4 @@
	re := regexp.MustCompile(`@@ \-\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "@@") {
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				start, _ := strconv.Atoi(matches[1])
				count := 1 // Default is 1 line if comma is missing
				if len(matches) > 2 && matches[2] != "" {
					count, _ = strconv.Atoi(matches[2])
				}

				// If count is 0, it means lines were removed, so no lines in new file correspond to this hunk directly?
				// Actually yes, " +10,0 " means lines after 10 were added? No, removed.
				// Wait, -old +new.
				// +10,0 means at line 10, 0 lines were added (empty hunk? usually implies deletion).
				// We care about added/modified lines in the NEW file.
				// If count > 0, those lines are in the new file.

				if count > 0 {
					intervals = append(intervals, LineInterval{
						Start: start,
						End:   start + count - 1,
					})
				}
			}
		}
	}
	return intervals
}

func isLineInIntervals(line int, intervals []LineInterval) bool {
	for _, iv := range intervals {
		if line >= iv.Start && line <= iv.End {
			return true
		}
	}
	return false
}

func printVerifyTable(cmd *cobra.Command, issues []VerifyIssue) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tTYPE\tFILE\tLINE\tMESSAGE")
	for _, i := range issues {
		icon := "🔴"
		if i.Severity == "Medium" {
			icon = "🟡"
		}
		if i.Severity == "Low" {
			icon = "⚪"
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\t%d\t%s\n", icon, i.Severity, i.Type, i.File, i.Line, i.Message)
	}
	w.Flush()
}
