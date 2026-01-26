package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	timelineDays   int
	timelineOutput string
	timelineFocus  string
)

var timelineCmd = &cobra.Command{
	Use:   "timeline",
	Short: "Visualize project timeline from git history",
	Long: `Analyze git commit history to create a visual timeline of features and fixes.
Groups commits by Conventional Commits scope (e.g., feat(auth) -> auth) or type.
Outputs Mermaid Gantt chart (default) or JSON.`,
	RunE: runTimeline,
}

func init() {
	rootCmd.AddCommand(timelineCmd)
	timelineCmd.Flags().IntVarP(&timelineDays, "days", "d", 30, "Number of days to look back")
	timelineCmd.Flags().StringVarP(&timelineOutput, "output", "o", "mermaid", "Output format (mermaid, json)")
	timelineCmd.Flags().StringVarP(&timelineFocus, "focus", "f", "", "Focus on a specific scope (substring)")
}

type TimelineEvent struct {
	Scope     string    `json:"scope"`
	Type      string    `json:"type"`
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Commits   []string  `json:"commits"`
}

func runTimeline(cmd *cobra.Command, args []string) error {
	gitClient := gitClientFactory()
	cwd := "." // Assume current working directory

	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// Calculate date
	sinceDate := time.Now().AddDate(0, 0, -timelineDays).Format("2006-01-02")
	sinceArg := fmt.Sprintf("--since=%s", sinceDate)

	// Fetch logs
	// Format: Hash|Date|Message
	// Use strict ISO 8601 date format for easier parsing
	logLines, err := gitClient.Log(cwd, "--pretty=format:%h|%aI|%s", sinceArg)
	if err != nil {
		return fmt.Errorf("failed to fetch git log: %w", err)
	}

	events := processCommits(logLines)

	// Filter by focus
	if timelineFocus != "" {
		var filtered []TimelineEvent
		for _, e := range events {
			if strings.Contains(strings.ToLower(e.Scope), strings.ToLower(timelineFocus)) {
				filtered = append(filtered, e)
			}
		}
		events = filtered
	}

	// Sort by StartDate
	sort.Slice(events, func(i, j int) bool {
		return events[i].StartDate.Before(events[j].StartDate)
	})

	switch strings.ToLower(timelineOutput) {
	case "json":
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	case "mermaid":
		output := generateMermaidGantt(events)
		fmt.Fprintln(cmd.OutOrStdout(), output)
	default:
		return fmt.Errorf("unknown format: %s", timelineOutput)
	}

	return nil
}

func processCommits(lines []string) []TimelineEvent {
	// Map: Scope -> Event
	eventMap := make(map[string]*TimelineEvent)

	// Regex for Conventional Commits
	// type(scope): subject
	// Group 1: type, Group 3: scope, Group 4: subject
	re := regexp.MustCompile(`^(\w+)(\(([\w\-\./]+)\))?:\s*(.+)$`)

	for _, line := range lines {
		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 3 {
			continue
		}
		// hash := parts[0]
		dateStr := parts[1]
		msg := parts[2]

		date, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			// Try fallback formats if needed, but strict ISO should work
			continue
		}

		// Parse message
		matches := re.FindStringSubmatch(msg)
		var scope, commitType string

		if len(matches) > 0 {
			commitType = matches[1]
			scope = matches[3]
		} else {
			commitType = "misc"
			scope = "general"
		}

		if scope == "" {
			scope = commitType // Fallback to type as scope
		}

		// Key for grouping
		key := scope

		if _, exists := eventMap[key]; !exists {
			eventMap[key] = &TimelineEvent{
				Scope:     scope,
				Type:      commitType,
				StartDate: date,
				EndDate:   date,
				Commits:   []string{},
			}
		}

		e := eventMap[key]
		if date.Before(e.StartDate) {
			e.StartDate = date
		}
		if date.After(e.EndDate) {
			e.EndDate = date
		}
		e.Commits = append(e.Commits, msg)
	}

	var events []TimelineEvent
	for _, e := range eventMap {
		// Extend end date by 1 day for visibility if start == end
		if e.StartDate.Equal(e.EndDate) {
			e.EndDate = e.EndDate.Add(24 * time.Hour)
		}
		events = append(events, *e)
	}

	return events
}

func generateMermaidGantt(events []TimelineEvent) string {
	var sb strings.Builder
	sb.WriteString("gantt\n")
	sb.WriteString("    title Project Timeline\n")
	sb.WriteString("    dateFormat YYYY-MM-DD\n")
	sb.WriteString("    axisFormat %Y-%m-%d\n")

	// Helper to format date
	fmtDate := func(t time.Time) string {
		return t.Format("2006-01-02")
	}

	for _, e := range events {
		safeTitle := strings.ReplaceAll(e.Scope, ":", " ")
		safeTitle = strings.ReplaceAll(safeTitle, "#", "")

		// Use section for grouping? Or just tasks?
		// Gantt tasks: taskName : criteria, start, end
		// If we use section per scope, it looks nice.
		sb.WriteString(fmt.Sprintf("    section %s\n", safeTitle))
		// Task name: Type (Count)
		taskName := fmt.Sprintf("%s (%d commits)", e.Type, len(e.Commits))
		sb.WriteString(fmt.Sprintf("    %s : %s, %s\n", taskName, fmtDate(e.StartDate), fmtDate(e.EndDate)))
	}

	return sb.String()
}
