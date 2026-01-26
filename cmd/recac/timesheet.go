package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	timesheetSince     string
	timesheetAuthor    string
	timesheetThreshold string
	timesheetPadding   string
	timesheetRate      float64
	timesheetJSON      bool
)

var timesheetCmd = &cobra.Command{
	Use:   "timesheet",
	Short: "Estimate work hours based on git history",
	Long: `Analyze git commit timestamps to estimate time spent working on the project.
Commits are grouped into "sessions". A session starts with a commit and ends when the gap to the next commit exceeds the threshold.
The duration of a session is (LastCommit - FirstCommit) + Padding.`,
	Example: `  recac timesheet --since 7d
  recac timesheet --author "jules" --rate 150
  recac timesheet --threshold 2h --padding 15m`,
	RunE: runTimesheet,
}

func init() {
	rootCmd.AddCommand(timesheetCmd)
	timesheetCmd.Flags().StringVar(&timesheetSince, "since", "24h", "Time window to analyze")
	timesheetCmd.Flags().StringVar(&timesheetAuthor, "author", "", "Filter by author (default: current user)")
	timesheetCmd.Flags().StringVar(&timesheetThreshold, "threshold", "60m", "Max gap between commits to consider one session")
	timesheetCmd.Flags().StringVar(&timesheetPadding, "padding", "30m", "Time assumed for a single commit or session start")
	timesheetCmd.Flags().Float64Var(&timesheetRate, "rate", 0, "Hourly rate for cost estimation")
	timesheetCmd.Flags().BoolVar(&timesheetJSON, "json", false, "Output results as JSON")
}

type Commit struct {
	Hash      string    `json:"hash"`
	Author    string    `json:"author"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

type Session struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  float64   `json:"duration_hours"` // in hours
	Commits   int       `json:"commits"`
}

type TimesheetReport struct {
	TotalHours    float64            `json:"total_hours"`
	TotalSessions int                `json:"total_sessions"`
	TotalCost     float64            `json:"total_cost,omitempty"`
	DailyStats    map[string]float64 `json:"daily_stats"`
	Sessions      []Session          `json:"sessions,omitempty"` // Only in JSON detailed? or maybe just keep it simple
}

func runTimesheet(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// 1. Resolve Defaults
	if timesheetAuthor == "" {
		author, err := getGitConfig(cwd, "user.name")
		if err != nil {
			return fmt.Errorf("could not detect git user.name: %w", err)
		}
		timesheetAuthor = author
	}

	threshold, err := time.ParseDuration(timesheetThreshold)
	if err != nil {
		return fmt.Errorf("invalid threshold: %w", err)
	}

	padding, err := time.ParseDuration(timesheetPadding)
	if err != nil {
		return fmt.Errorf("invalid padding: %w", err)
	}

	// 2. Fetch Commits
	commits, err := getGitCommits(cwd, timesheetSince, timesheetAuthor)
	if err != nil {
		return err
	}

	if len(commits) == 0 {
		if timesheetJSON {
			fmt.Println("{}")
		} else {
			fmt.Printf("No commits found for author '%s' since %s.\n", timesheetAuthor, timesheetSince)
		}
		return nil
	}

	// 3. Calculate Sessions
	sessions := calculateSessions(commits, threshold, padding)

	// 4. Aggregate Stats
	report := aggregateTimesheet(sessions, timesheetRate)

	// 5. Output
	if timesheetJSON {
		// Include sessions in JSON output
		report.Sessions = sessions
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}

	printTimesheetTable(report, timesheetAuthor, timesheetSince, timesheetRate)
	return nil
}

func getGitConfig(dir, key string) (string, error) {
	client := gitClientFactory()
	out, err := client.Run(dir, "config", key)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func getGitCommits(dir, since, author string) ([]Commit, error) {
	// Format: Hash|Author|ISO8601|Message
	args := []string{"log", "--since=" + since, "--format=%h|%an|%aI|%s"}
	if author != "" {
		args = append(args, "--author="+author)
	}

	client := gitClientFactory()
	lines, err := client.Log(dir, args...)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	return parseGitLogOutput(lines)
}

func parseGitLogOutput(lines []string) ([]Commit, error) {
	var commits []Commit
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		t, err := time.Parse(time.RFC3339, parts[2])
		if err != nil {
			continue
		}
		commits = append(commits, Commit{
			Hash:      parts[0],
			Author:    parts[1],
			Timestamp: t,
			Message:   parts[3],
		})
	}

	// Sort by time ascending (git log is usually descending)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Timestamp.Before(commits[j].Timestamp)
	})

	return commits, nil
}

func calculateSessions(commits []Commit, threshold, padding time.Duration) []Session {
	if len(commits) == 0 {
		return nil
	}

	var sessions []Session
	currentStart := commits[0].Timestamp
	currentEnd := commits[0].Timestamp
	count := 1

	for i := 1; i < len(commits); i++ {
		t := commits[i].Timestamp
		gap := t.Sub(currentEnd)

		if gap <= threshold {
			// Continue session
			currentEnd = t
			count++
		} else {
			// End session
			duration := currentEnd.Sub(currentStart) + padding
			sessions = append(sessions, Session{
				StartTime: currentStart,
				EndTime:   currentEnd, // Actual last commit time
				Duration:  duration.Hours(),
				Commits:   count,
			})

			// Start new session
			currentStart = t
			currentEnd = t
			count = 1
		}
	}

	// Push last session
	duration := currentEnd.Sub(currentStart) + padding
	sessions = append(sessions, Session{
		StartTime: currentStart,
		EndTime:   currentEnd,
		Duration:  duration.Hours(),
		Commits:   count,
	})

	return sessions
}

func aggregateTimesheet(sessions []Session, rate float64) TimesheetReport {
	report := TimesheetReport{
		DailyStats: make(map[string]float64),
	}

	for _, s := range sessions {
		report.TotalSessions++
		report.TotalHours += s.Duration
		date := s.StartTime.Format("2006-01-02")
		report.DailyStats[date] += s.Duration
	}

	if rate > 0 {
		report.TotalCost = report.TotalHours * rate
	}

	return report
}

func printTimesheetTable(report TimesheetReport, author, since string, rate float64) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintf(w, "Timesheet Report\n")
	fmt.Fprintf(w, "Author: %s\n", author)
	fmt.Fprintf(w, "Period: Since %s\n", since)
	fmt.Fprintf(w, "------------------------------------------------\n")

	// Sort dates
	var dates []string
	for d := range report.DailyStats {
		dates = append(dates, d)
	}
	sort.Strings(dates)

	for _, d := range dates {
		h := report.DailyStats[d]
		fmt.Fprintf(w, "%s\t%.2f hrs\n", d, h)
	}

	fmt.Fprintf(w, "------------------------------------------------\n")
	fmt.Fprintf(w, "Total Sessions:\t%d\n", report.TotalSessions)
	fmt.Fprintf(w, "Total Hours:\t%.2f hrs\n", report.TotalHours)
	if rate > 0 {
		fmt.Fprintf(w, "Estimated Cost:\t$%.2f (@ $%.0f/hr)\n", report.TotalCost, rate)
	}
	w.Flush()
}
