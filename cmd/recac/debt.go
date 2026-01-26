package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	debtJSON   bool
	debtMinAge string
	debtAuthor string
)

var debtCmd = &cobra.Command{
	Use:   "debt [path]",
	Short: "Analyze technical debt age using git blame",
	Long: `Scans for TODOs and FIXMEs, then runs git blame to determine how old they are.
It helps identify abandoned tasks and long-standing technical debt.`,
	RunE: runDebt,
}

func init() {
	rootCmd.AddCommand(debtCmd)
	debtCmd.Flags().BoolVar(&debtJSON, "json", false, "Output results as JSON")
	debtCmd.Flags().StringVar(&debtMinAge, "min-age", "", "Filter tasks older than duration (e.g. 1y, 6m, 30d)")
	debtCmd.Flags().StringVar(&debtAuthor, "author", "", "Filter tasks by author name (substring match)")
}

type DebtItem struct {
	TodoItem
	Author     string    `json:"author"`
	CommitHash string    `json:"commit"`
	Date       time.Time `json:"date"`
	Age        string    `json:"age"` // Human readable age
}

func runDebt(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Scan for TODOs
	todos, err := ScanForTodos(root)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if len(todos) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No TODOs found.")
		return nil
	}

	// 2. Parse min-age if provided
	var minAgeTime time.Time
	if debtMinAge != "" {
		d, err := parseDurationExtended(debtMinAge)
		if err != nil {
			return fmt.Errorf("invalid min-age: %w", err)
		}
		minAgeTime = time.Now().Add(-d)
	}

	// 3. Enrich with Git Blame
	var debtItems []DebtItem

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not installed or not in PATH")
	}

	for _, todo := range todos {
		blame, err := gitBlameLine(todo.File, todo.Line)
		if err != nil {
			// If blame fails (e.g. file not tracked), skip or add placeholder
			// fmt.Fprintf(cmd.ErrOrStderr(), "Warning: blame failed for %s:%d: %v\n", todo.File, todo.Line, err)
			continue
		}

		// Filter by Author
		if debtAuthor != "" && !strings.Contains(strings.ToLower(blame.Author), strings.ToLower(debtAuthor)) {
			continue
		}

		// Filter by Age
		if !minAgeTime.IsZero() && blame.Date.After(minAgeTime) {
			continue
		}

		item := DebtItem{
			TodoItem:   todo,
			Author:     blame.Author,
			CommitHash: blame.CommitHash,
			Date:       blame.Date,
			Age:        timeSinceHuman(blame.Date),
		}
		debtItems = append(debtItems, item)
	}

	// 4. Sort by Date (Oldest first)
	sort.Slice(debtItems, func(i, j int) bool {
		return debtItems[i].Date.Before(debtItems[j].Date)
	})

	// 5. Output
	if debtJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(debtItems)
	}

	printDebtTable(cmd, debtItems)
	return nil
}

type BlameResult struct {
	Author     string
	Date       time.Time
	CommitHash string
}

func gitBlameLine(file string, line int) (*BlameResult, error) {
	// git blame -L n,n --porcelain file
	// execCommand is defined in resume.go (shared package var)
	cmd := execCommand("git", "blame", "-L", fmt.Sprintf("%d,%d", line, line), "--porcelain", file)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	return parseBlamePorcelain(out.String())
}

func parseBlamePorcelain(output string) (*BlameResult, error) {
	lines := strings.Split(output, "\n")
	res := &BlameResult{}

	// First line is commit hash + line info
	// 4e5d6f... 1 1 1
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) > 0 {
			res.CommitHash = parts[0]
		}
	}

	for _, l := range lines {
		if strings.HasPrefix(l, "author ") {
			res.Author = strings.TrimPrefix(l, "author ")
		} else if strings.HasPrefix(l, "author-time ") {
			tsStr := strings.TrimPrefix(l, "author-time ")
			ts, err := strconv.ParseInt(tsStr, 10, 64)
			if err == nil {
				res.Date = time.Unix(ts, 0)
			}
		}
	}

	return res, nil
}

func printDebtTable(cmd *cobra.Command, items []DebtItem) {
	if len(items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matching technical debt found.")
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Age\tAuthor\tLocation\tTask")
	fmt.Fprintln(w, "---\t------\t--------\t----")

	for _, item := range items {
		loc := fmt.Sprintf("%s:%d", item.File, item.Line)
		content := item.Content
		if len(content) > 50 {
			content = content[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", item.Age, item.Author, loc, content)
	}
	w.Flush()
	fmt.Fprintf(cmd.OutOrStdout(), "\nFound %d items.\n", len(items))
}

func timeSinceHuman(t time.Time) string {
	d := time.Since(t)
	days := int(d.Hours() / 24)

	if days > 365 {
		years := float64(days) / 365.0
		return fmt.Sprintf("%.1fy", years)
	}
	if days > 30 {
		months := float64(days) / 30.0
		return fmt.Sprintf("%.1fm", months)
	}
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	return "today"
}

func parseDurationExtended(s string) (time.Duration, error) {
	// Simple extension for d, w, m, y
	if strings.HasSuffix(s, "y") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "y"))
		if err != nil {
			return 0, err
		}
		return time.Hour * 24 * 365 * time.Duration(val), nil
	}
	if strings.HasSuffix(s, "m") { // months
		val, err := strconv.Atoi(strings.TrimSuffix(s, "m"))
		if err != nil {
			return 0, err
		}
		return time.Hour * 24 * 30 * time.Duration(val), nil
	}
	if strings.HasSuffix(s, "w") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "w"))
		if err != nil {
			return 0, err
		}
		return time.Hour * 24 * 7 * time.Duration(val), nil
	}
	if strings.HasSuffix(s, "d") {
		val, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, err
		}
		return time.Hour * 24 * time.Duration(val), nil
	}
	return time.ParseDuration(s)
}
