package main

import (
	"bytes"
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(summaryCmd)
	summaryCmd.Flags().Int("limit", 5, "Limit for 'Recent Sessions' and 'Top Sessions by Cost' lists")
}

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Display a consolidated summary of RECAC status, stats, and activity",
	Long:  `Provides a high-level dashboard view of the most important information, including system status, aggregate stats, recent sessions, and cost highlights.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		limit, _ := cmd.Flags().GetInt("limit")
		return doSummary(cmd, sm, limit)
	},
}

// doSummary orchestrates the gathering of data from various sources and displays the summary.
// It is decoupled from the cobra command to facilitate testing.
func doSummary(cmd *cobra.Command, sm ISessionManager, limit int) error {
	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("could not list sessions: %w", err)
	}

	// --- 1. System Status ---
	// Reuse the same status logic as the `status` command.
	statusOutput := ui.GetStatus()
	cmd.Println(statusOutput)

	// If there are no sessions, we can stop here.
	if len(sessions) == 0 {
		cmd.Println("No session activity found.")
		return nil
	}

	// --- 2. Aggregate Stats ---
	// Reuse the `calculateStats` logic from the `stats` command.
	stats, err := calculateStats(sm)
	if err != nil {
		return fmt.Errorf("could not calculate statistics: %w", err)
	}
	cmd.Println("📊 Aggregate Stats")
	cmd.Println("-------------------")
	var statsBuf bytes.Buffer
	wStats := tabwriter.NewWriter(&statsBuf, 0, 0, 2, ' ', 0)
	fmt.Fprintf(wStats, "Total Sessions:\t%d\n", stats.TotalSessions)
	fmt.Fprintf(wStats, "Total Cost:\t$%.4f\n", stats.TotalCost)
	fmt.Fprintf(wStats, "Total Tokens:\t%d\n", stats.TotalTokens)
	wStats.Flush()
	cmd.Println(statsBuf.String())

	// --- 3. Recent Sessions ---
	// Sort sessions by start time (most recent first) for the "Recent Activity" list.
	// This is similar to the default sort in the `ps` command.
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
	cmd.Println("⏳ Recent Activity")
	cmd.Println("-------------------")
	var recentBuf bytes.Buffer
	wRecent := tabwriter.NewWriter(&recentBuf, 0, 0, 2, ' ', 0)
	fmt.Fprintln(wRecent, "Name\tStatus\tStarted\tDuration")
	for i, session := range sessions {
		if i >= limit {
			break
		}
		started := session.StartTime.Format("15:04:05") // More compact for summary
		var duration string
		if session.EndTime.IsZero() {
			duration = time.Since(session.StartTime).Round(time.Second).String()
		} else {
			duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
		}
		fmt.Fprintf(wRecent, "%s\t%s\t%s\t%s\n",
			truncateString(session.Name, 30),
			session.Status,
			started,
			duration)
	}
	wRecent.Flush()
	cmd.Println(recentBuf.String())

	// --- 4. Top Sessions by Cost ---
	// Reuse the `analyzeSessionCosts` logic from the `cost` command.
	costAnalysis, err := analyzeSessionCosts(sessions, limit)
	if err != nil {
		return fmt.Errorf("could not analyze costs: %w", err)
	}

	// Only show this section if there's actual cost data.
	if len(costAnalysis.TopSessionsByCost) > 0 {
		cmd.Println("💰 Top Sessions by Cost")
		cmd.Println("------------------------")
		var costBuf bytes.Buffer
		wCost := tabwriter.NewWriter(&costBuf, 0, 0, 2, ' ', 0)
		fmt.Fprintln(wCost, "Session Name\tModel\tCost")
		for _, s := range costAnalysis.TopSessionsByCost {
			fmt.Fprintf(wCost, "%s\t%s\t$%.6f\n",
				truncateString(s.Name, 30),
				s.Model,
				s.Cost)
		}
		wCost.Flush()
		cmd.Println(costBuf.String())
	}

	return nil
}

// truncateString is a small helper to keep table output clean.
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
