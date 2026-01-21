package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"
)

var (
	covThreshold      float64
	covPatchThreshold float64
	covProfile        string
	covRunCmd         string
	covKeepProfile    bool
	covHtml           string
)

var coverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Measure code coverage, focusing on changed lines (Patch Coverage)",
	Long: `Runs tests with coverage and calculates the coverage percentage.
It also calculates "Patch Coverage" - the percentage of *new or modified* lines that are covered by tests.
This is critical for ensuring that new code is adequately tested.

If --run-cmd is not provided, it defaults to 'go test -coverprofile=coverage.out ./...'.
`,
	RunE: runCoverage,
}

func init() {
	rootCmd.AddCommand(coverageCmd)
	coverageCmd.Flags().Float64Var(&covThreshold, "threshold", 0, "Minimum total coverage percentage required to pass")
	coverageCmd.Flags().Float64Var(&covPatchThreshold, "patch-threshold", 75.0, "Minimum patch coverage percentage required to pass")
	coverageCmd.Flags().StringVar(&covProfile, "profile", "coverage.out", "Path to output coverage profile")
	coverageCmd.Flags().StringVar(&covRunCmd, "run-cmd", "", "Command to run tests (default: go test -coverprofile=<profile> ./...)")
	coverageCmd.Flags().BoolVar(&covKeepProfile, "keep", false, "Keep the coverage profile file after running")
	coverageCmd.Flags().StringVar(&covHtml, "html", "", "Generate HTML coverage report to this file")
}

// coverageExec allows mocking exec.Command in tests
var coverageExec = exec.Command

// CoverageLineInterval represents a range of lines
type CoverageLineInterval struct {
	Start int
	End   int
}

func runCoverage(cmd *cobra.Command, args []string) error {
	// 1. Run Tests
	runCommand := covRunCmd
	if runCommand == "" {
		runCommand = fmt.Sprintf("go test -coverprofile=%s ./...", covProfile)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🏃 Running tests: %s\n", runCommand)

	// Split command using shellquote to handle spaces and quotes correctly
	parts, err := shellquote.Split(runCommand)
	if err != nil {
		return fmt.Errorf("failed to parse run command: %w", err)
	}

	if len(parts) == 0 {
		return fmt.Errorf("empty run command")
	}

	c := coverageExec(parts[0], parts[1:]...)
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()

	if err := c.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	if !covKeepProfile {
		defer os.Remove(covProfile)
	}

	// 2. Parse Coverage Profile
	profileData, err := parseCoverageProfile(covProfile)
	if err != nil {
		return fmt.Errorf("failed to parse coverage profile: %w", err)
	}

	// 3. Get Changed Lines (Patch)
	changedLines, err := getPatchIntervals()
	if err != nil {
		// If not a git repo or error, warn but proceed with total coverage only?
		// Or fail? Let's warn.
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not calculate patch coverage (git diff failed): %v\n", err)
	}

	// 4. Calculate Stats
	totalCov := calculateTotalCoverage(profileData)
	patchCov, hasPatchData := calculatePatchCoverage(profileData, changedLines)

	// 5. Generate HTML if requested
	if covHtml != "" {
		if err := generateHtmlReport(covProfile, covHtml); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to generate HTML report: %v\n", err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "📄 HTML report generated: %s\n", covHtml)
		}
	}

	// 6. Report
	printCoverageReport(cmd.OutOrStdout(), totalCov, patchCov, hasPatchData)

	// 7. Check Thresholds
	var failures []string

	if covThreshold > 0 && totalCov < covThreshold {
		failures = append(failures, fmt.Sprintf("Total coverage %.1f%% is below threshold %.1f%%", totalCov, covThreshold))
	}

	if hasPatchData && patchCov < covPatchThreshold {
		// Only enforce patch threshold if we actually have patch data (changes exist)
		failures = append(failures, fmt.Sprintf("Patch coverage %.1f%% is below threshold %.1f%%", patchCov, covPatchThreshold))
	}

	if len(failures) > 0 {
		return fmt.Errorf("coverage checks failed:\n- %s", strings.Join(failures, "\n- "))
	}

	return nil
}

type CoverBlock struct {
	StartLine int
	EndLine   int
	NumStmt   int
	Count     int
}

type ProfileData map[string][]CoverBlock

func parseCoverageProfile(path string) (ProfileData, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data := make(ProfileData)
	scanner := bufio.NewScanner(f)
	// format: name.go:line.col,line.col numstmt count
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		file := parts[0]
		rest := parts[1]

		fields := strings.Fields(rest)
		if len(fields) != 3 {
			continue
		}

		// 10.22,12.33
		rangeParts := strings.Split(fields[0], ",")
		if len(rangeParts) != 2 {
			continue
		}

		startParts := strings.Split(rangeParts[0], ".")
		endParts := strings.Split(rangeParts[1], ".")

		startLine, _ := strconv.Atoi(startParts[0])
		endLine, _ := strconv.Atoi(endParts[0])
		numStmt, _ := strconv.Atoi(fields[1])
		count, _ := strconv.Atoi(fields[2])

		data[file] = append(data[file], CoverBlock{
			StartLine: startLine,
			EndLine:   endLine,
			NumStmt:   numStmt,
			Count:     count,
		})
	}
	return data, scanner.Err()
}

func getPatchIntervals() (map[string][]CoverageLineInterval, error) {
	// git diff --unified=0 HEAD
	cmd := coverageExec("git", "diff", "--unified=0", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		// Fallback for no HEAD
		cmd = coverageExec("git", "diff", "--unified=0")
		out, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	return parseDiffToMap(string(out)), nil
}

func parseDiffToMap(diff string) map[string][]CoverageLineInterval {
	result := make(map[string][]CoverageLineInterval)
	var currentFile string

	scanner := bufio.NewScanner(strings.NewReader(diff))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "diff --git") {
			// diff --git a/file.go b/file.go
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				// b/file.go
				pathB := parts[len(parts)-1]
				if strings.HasPrefix(pathB, "b/") {
					currentFile = strings.TrimPrefix(pathB, "b/")
				} else {
					currentFile = pathB
				}
			}
		} else if strings.HasPrefix(line, "+++ ") {
			// +++ b/file.go
			// Confirmation of file name (sometimes better than diff --git line)
			path := strings.TrimPrefix(line, "+++ ")
			if strings.HasPrefix(path, "b/") {
				currentFile = strings.TrimPrefix(path, "b/")
			} else {
				// Sometimes it's just the filename
			}
		} else if strings.HasPrefix(line, "@@") {
			// @@ -1,2 +3,4 @@
			re := regexp.MustCompile(`@@ \-\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				start, _ := strconv.Atoi(matches[1])
				count := 1
				if len(matches) > 2 && matches[2] != "" {
					count, _ = strconv.Atoi(matches[2])
				}
				if count > 0 {
					result[currentFile] = append(result[currentFile], CoverageLineInterval{
						Start: start,
						End:   start + count - 1,
					})
				}
			}
		}
	}
	return result
}

func calculateTotalCoverage(data ProfileData) float64 {
	var totalStmt, coveredStmt int
	for _, blocks := range data {
		for _, b := range blocks {
			totalStmt += b.NumStmt
			if b.Count > 0 {
				coveredStmt += b.NumStmt
			}
		}
	}
	if totalStmt == 0 {
		return 100.0 // Empty project is covered? Or 0? 100 prevents false failure.
	}
	return float64(coveredStmt) / float64(totalStmt) * 100.0
}

func calculatePatchCoverage(data ProfileData, patch map[string][]CoverageLineInterval) (float64, bool) {
	var totalStmt, coveredStmt int
	hasChanges := false

	for file, intervals := range patch {
		// Coverage profile usually has full paths or relative to module root.
		// Git diff has paths relative to repo root.
		// We try to match them.

		// In Go coverage, file paths are usually "module/path/to/file.go"
		// Git is "path/to/file.go"
		// We need to match loosely.

		var fileBlocks []CoverBlock
		// Find matching file in coverage data
		for covFile, blocks := range data {
			if strings.HasSuffix(covFile, file) {
				fileBlocks = blocks
				break
			}
		}

		if len(fileBlocks) == 0 {
			continue // No coverage data for this changed file
		}

		for _, interval := range intervals {
			hasChanges = true
			for _, b := range fileBlocks {
				// Check overlap
				if b.EndLine < interval.Start || b.StartLine > interval.End {
					continue
				}

				totalStmt += b.NumStmt
				if b.Count > 0 {
					coveredStmt += b.NumStmt
				}
			}
		}
	}

	if totalStmt == 0 {
		if hasChanges {
			return 100.0, false
		}
		return 0.0, false
	}

	return float64(coveredStmt) / float64(totalStmt) * 100.0, true
}

func generateHtmlReport(profile, output string) error {
	cmd := coverageExec("go", "tool", "cover", fmt.Sprintf("-html=%s", profile), fmt.Sprintf("-o=%s", output))
	return cmd.Run()
}

func printCoverageReport(w io.Writer, total, patch float64, hasPatch bool) {
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "\nCOVERAGE REPORT")
	fmt.Fprintln(tw, "---------------")

	fmt.Fprintf(tw, "Total Coverage:\t%.1f%%\n", total)

	patchStr := "N/A (no changes)"
	if hasPatch {
		patchStr = fmt.Sprintf("%.1f%%", patch)
	}
	fmt.Fprintf(tw, "Patch Coverage:\t%s\n", patchStr)

	tw.Flush()
}
