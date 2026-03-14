package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"recac/internal/security"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	securityHistoryCommits int
	securityHistoryAll     bool
	securityHistoryJSON    bool
	securityHistoryFail    bool

	// Regex for hunk header: @@ -old,n +new,n @@
	hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
)

type HistorySecurityResult struct {
	Commit      string `json:"commit"`
	Author      string `json:"author"`
	Date        string `json:"date"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Match       string `json:"match"`
}

var securityHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Scan git history for secrets",
	Long:  `Scans the git history (diffs) for secrets that may have been committed in the past.`,
	RunE:  runSecurityHistoryScan,
}

func init() {
	securityCmd.AddCommand(securityHistoryCmd)
	securityHistoryCmd.Flags().IntVar(&securityHistoryCommits, "commits", 50, "Number of recent commits to scan")
	securityHistoryCmd.Flags().BoolVar(&securityHistoryAll, "all", false, "Scan entire history (overrides --commits)")
	securityHistoryCmd.Flags().BoolVar(&securityHistoryJSON, "json", false, "Output results as JSON")
	securityHistoryCmd.Flags().BoolVar(&securityHistoryFail, "fail", false, "Exit with error code if secrets are found")
}

func runSecurityHistoryScan(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Check if git repo
	if _, err := os.Stat(".git"); os.IsNotExist(err) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// 2. Prepare git log command
	gitArgs := []string{"log", "-p", "--no-color", "--unified=0"}
	if !securityHistoryAll {
		gitArgs = append(gitArgs, fmt.Sprintf("-n%d", securityHistoryCommits))
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Scanning git history (%s)...\n", func() string {
		if securityHistoryAll {
			return "all commits"
		}
		return fmt.Sprintf("last %d commits", securityHistoryCommits)
	}())

	// Run git log
	execCmd := exec.Command("git", gitArgs...)
	execCmd.Dir = cwd

	stdout, err := execCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := execCmd.Start(); err != nil {
		return fmt.Errorf("failed to start git log: %w", err)
	}

	// 3. Parse Output
	results := parseGitLogStream(stdout)

	if err := execCmd.Wait(); err != nil {
		// git log might exit with error if pipe closed early, or just normal exit
		// If results found, maybe ignore? But strictly we should report error.
		return fmt.Errorf("git log failed: %w", err)
	}

	// 4. Report
	if len(results) == 0 {
		if !securityHistoryJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ No secrets found in history.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil
	}

	if securityHistoryJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	printHistorySecurityTable(cmd, results)

	if securityHistoryFail {
		return fmt.Errorf("found %d secrets in history", len(results))
	}

	return nil
}

func parseGitLogStream(r io.Reader) []HistorySecurityResult {
	securityScanner := security.NewRegexScanner()
	scanner := bufio.NewScanner(r)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	var results []HistorySecurityResult

	var currentCommit string
	var currentAuthor string
	var currentDate string
	var currentFile string
	var currentLine int

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "commit ") {
			currentCommit = strings.TrimPrefix(line, "commit ")
			currentFile = ""
			currentLine = 0
			continue
		}
		if strings.HasPrefix(line, "Author: ") {
			currentAuthor = strings.TrimPrefix(line, "Author: ")
			continue
		}
		if strings.HasPrefix(line, "Date: ") {
			currentDate = strings.TrimPrefix(line, "Date: ")
			continue
		}
		if strings.HasPrefix(line, "diff --git ") {
			// diff --git a/file b/file
			// Don't rely on this for filename if spaces are present, wait for +++ line
			// But set a default in case +++ is missing (e.g. binary files?)
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				currentFile = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}
		if strings.HasPrefix(line, "+++ ") {
			// +++ b/file
			// This is more reliable for new file path
			// Remove prefix "+++ b/" or "+++ "
			path := strings.TrimPrefix(line, "+++ ")
			// Remove quotes if present
			path = strings.Trim(path, "\"")

			if strings.HasPrefix(path, "b/") {
				currentFile = strings.TrimPrefix(path, "b/")
			} else {
				currentFile = path
			}
			continue
		}
		if strings.HasPrefix(line, "--- ") {
			continue
		}
		if strings.HasPrefix(line, "index ") {
			continue
		}

		if strings.HasPrefix(line, "@@ ") {
			// Parse hunk header
			matches := hunkRe.FindStringSubmatch(line)
			if len(matches) > 2 {
				// new start line
				start, _ := strconv.Atoi(matches[2])
				currentLine = start
			}
			continue
		}

		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			// Added line
			content := strings.TrimPrefix(line, "+")
			findings, _ := securityScanner.Scan(content)
			for _, f := range findings {
				results = append(results, HistorySecurityResult{
					Commit:      currentCommit,
					Author:      strings.TrimSpace(currentAuthor),
					Date:        strings.TrimSpace(currentDate),
					File:        currentFile,
					Line:        currentLine,
					Type:        f.Type,
					Description: f.Description,
					Match:       f.Match,
				})
			}
			currentLine++
		} else if strings.HasPrefix(line, " ") {
			// Context line
			currentLine++
		}
	}

	return results
}

func printHistorySecurityTable(cmd *cobra.Command, results []HistorySecurityResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "COMMIT\tDATE\tFILE\tTYPE\tMATCH")
	fmt.Fprintln(w, "------\t----\t----\t----\t-----")

	for _, r := range results {
		shortCommit := r.Commit
		if len(shortCommit) > 7 {
			shortCommit = shortCommit[:7]
		}

		// Parse date to shorter format?
		// Git date format: "Fri Feb 21 14:08:35 2025 +0100"
		// Just keep it or truncate?
		// Let's truncate to first 2 words (Day Month Day)
		// Or just use the string.

		fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\t%s\n", shortCommit, r.Date, r.File, r.Line, r.Type, r.Match)
	}
	w.Flush()
	fmt.Fprintf(cmd.OutOrStdout(), "\nFound %d potential secrets in history.\n", len(results))
}
