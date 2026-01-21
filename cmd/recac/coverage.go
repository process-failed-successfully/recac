package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/spf13/cobra"
)

// coverageExec allows mocking exec.Command in tests
var coverageExec = exec.Command

var (
	coverageThreshold float64
	coverageHTML      bool
	coverageBranch    string
	coverageRunCmd    string
)

var coverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Check patch coverage for modified code",
	Long: `Calculates "Patch Coverage" - the percentage of *new or modified* lines that are covered by tests.
It runs tests with coverage enabled, parses the git diff to find changed lines, and intersects the two.

This ensures that you are not adding new technical debt by merging untested code.`,
	RunE: runCoverage,
}

func init() {
	rootCmd.AddCommand(coverageCmd)
	coverageCmd.Flags().Float64VarP(&coverageThreshold, "threshold", "t", 80.0, "Minimum patch coverage percentage required")
	coverageCmd.Flags().BoolVar(&coverageHTML, "html", false, "Open the HTML coverage report if available")
	coverageCmd.Flags().StringVarP(&coverageBranch, "branch", "b", "HEAD", "Target branch/commit to diff against")
	coverageCmd.Flags().StringVar(&coverageRunCmd, "run-cmd", "go test ./... -coverprofile=coverage.out", "Command to run tests with coverage")
}

func runCoverage(cmd *cobra.Command, args []string) error {
	// 1. Get Modified Lines
	diffFileMap, err := getModifiedLines(coverageBranch)
	if err != nil {
		return fmt.Errorf("failed to get modified lines: %w", err)
	}

	if len(diffFileMap) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No modified go files found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found changes in %d files.\n", len(diffFileMap))

	// 2. Run Tests
	fmt.Fprintf(cmd.OutOrStdout(), "Running tests: %s\n", coverageRunCmd)
	parts, err := shellquote.Split(coverageRunCmd)
	if err != nil {
		return fmt.Errorf("failed to parse run-cmd: %w", err)
	}

	if len(parts) == 0 {
		return fmt.Errorf("empty run-cmd")
	}

	testCmd := coverageExec(parts[0], parts[1:]...)
	testCmd.Stdout = cmd.OutOrStdout()
	testCmd.Stderr = cmd.ErrOrStderr()

	if err := testCmd.Run(); err != nil {
		return fmt.Errorf("tests failed: %w", err)
	}

	// Ensure cleanup
	defer func() {
		if !coverageHTML {
			// Only remove if it exists and we created it (assumed)
			os.Remove("coverage.out")
		}
	}()

	// 3. Parse Coverage
	coverageProfilePath := "coverage.out"
	// Try to find if user specified a different profile in run-cmd
	if strings.Contains(coverageRunCmd, "-coverprofile=") {
		re := regexp.MustCompile(`-coverprofile=(\S+)`)
		matches := re.FindStringSubmatch(coverageRunCmd)
		if len(matches) > 1 {
			coverageProfilePath = matches[1]
		}
	} else if strings.Contains(coverageRunCmd, "-coverprofile ") {
		parts := strings.Fields(coverageRunCmd)
		for i, p := range parts {
			if p == "-coverprofile" && i+1 < len(parts) {
				coverageProfilePath = parts[i+1]
				break
			}
		}
	}

	profiles, err := parseCoverageProfile(coverageProfilePath)
	if err != nil {
		return fmt.Errorf("failed to parse coverage profile (%s): %w", coverageProfilePath, err)
	}

	// 4. Calculate Patch Coverage
	coveredLines := 0
	totalModifiedLines := 0
	uncovered := make(map[string][]int)

	for file, lines := range diffFileMap {
		// Profile paths might be full or relative.
		// Go coverprofile usually has "module/path/to/file.go".
		// git diff has "path/to/file.go".
		// We need to match suffix.

		var fileProfile *Profile
		for pFile, prof := range profiles {
			if strings.HasSuffix(pFile, file) {
				// We need to take the address of the loop variable's value
				// But loop variable 'prof' changes.
				// Better:
				p := prof
				fileProfile = &p
				break
			}
		}

		for _, line := range lines {
			totalModifiedLines++
			if fileProfile == nil {
				uncovered[file] = append(uncovered[file], line)
				continue
			}

			if isCovered(fileProfile, line) {
				coveredLines++
			} else {
				uncovered[file] = append(uncovered[file], line)
			}
		}
	}

	// 5. Report
	percentage := 0.0
	if totalModifiedLines > 0 {
		percentage = (float64(coveredLines) / float64(totalModifiedLines)) * 100
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nPatch Coverage: %.1f%% (%d/%d lines)\n", percentage, coveredLines, totalModifiedLines)

	if len(uncovered) > 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "\nUntested Modified Lines:")
		for file, lines := range uncovered {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s: %v\n", file, lines)
		}
	}

	if coverageHTML {
		// Best effort
		coverageExec("go", "tool", "cover", "-html="+coverageProfilePath).Start()
	}

	if percentage < coverageThreshold {
		return fmt.Errorf("patch coverage %.1f%% is below threshold %.1f%%", percentage, coverageThreshold)
	}

	return nil
}

// --- Helpers ---

// map[filename][]lineNumbers
func getModifiedLines(branch string) (map[string][]int, error) {
	// git diff --unified=0 HEAD
	// We use 0 context to make parsing easier (only changed lines)
	cmd := coverageExec("git", "diff", "--unified=0", branch)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseGitDiff(string(out)), nil
}

func parseGitDiff(diff string) map[string][]int {
	res := make(map[string][]int)
	scanner := bufio.NewScanner(strings.NewReader(diff))

	var currentFile string
	var currentLine int

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "diff --git") {
			continue
		}
		if strings.HasPrefix(line, "+++ b/") {
			currentFile = strings.TrimPrefix(line, "+++ b/")
			continue
		}
		if strings.HasPrefix(line, "--- a/") {
			continue
		}

		// Chunk header: @@ -old,cnt +new,cnt @@
		// Or: @@ -10 +20 @@
		if strings.HasPrefix(line, "@@") {
			parts := strings.Split(line, " ")
			if len(parts) >= 3 {
				newPart := parts[2] // +20,5 or +20
				newPart = strings.TrimPrefix(newPart, "+")
				comma := strings.Index(newPart, ",")
				if comma != -1 {
					start, _ := strconv.Atoi(newPart[:comma])
					currentLine = start
				} else {
					start, _ := strconv.Atoi(newPart)
					currentLine = start
				}
			}
			continue
		}

		if currentFile == "" {
			continue
		}

		if strings.HasSuffix(currentFile, "_test.go") || !strings.HasSuffix(currentFile, ".go") {
			continue
		}

		if strings.HasPrefix(line, "+") {
			res[currentFile] = append(res[currentFile], currentLine)
			currentLine++
		} else if strings.HasPrefix(line, "-") {
			// Deleted line, ignore
		} else {
			// Context line (if any)
			// currentLine++
			// Wait, unified=0 should have NO context lines unless we misconfigured.
			// But if manual diff has context, we should increment.
			// How do we distinguish context from just a line starting with space?
			// Unified diff context starts with space.
			if strings.HasPrefix(line, " ") {
				currentLine++
			}
		}
	}
	return res
}

type Profile struct {
	Blocks []ProfileBlock
}
type ProfileBlock struct {
	StartLine int
	EndLine   int
	Count     int
}

func parseCoverageProfile(path string) (map[string]Profile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	res := make(map[string]Profile)
	scanner := bufio.NewScanner(f)
	// mode: set
	if scanner.Scan() {
		_ = scanner.Text() // skip mode
	}

	for scanner.Scan() {
		line := scanner.Text()
		// format: name.go:line.col,line.col numstmt count
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		fileName := parts[0]

		rest := parts[1]
		fields := strings.Fields(rest)
		if len(fields) != 3 {
			continue
		}

		rangeParts := strings.Split(fields[0], ",")
		if len(rangeParts) != 2 {
			continue
		}

		startParts := strings.Split(rangeParts[0], ".")
		endParts := strings.Split(rangeParts[1], ".")

		startLine, _ := strconv.Atoi(startParts[0])
		endLine, _ := strconv.Atoi(endParts[0])
		count, _ := strconv.Atoi(fields[2])

		prof := res[fileName]
		prof.Blocks = append(prof.Blocks, ProfileBlock{
			StartLine: startLine,
			EndLine:   endLine,
			Count:     count,
		})
		res[fileName] = prof
	}

	return res, nil
}

func isCovered(p *Profile, line int) bool {
	for _, b := range p.Blocks {
		if line >= b.StartLine && line <= b.EndLine {
			if b.Count > 0 {
				return true
			}
		}
	}
	return false
}
