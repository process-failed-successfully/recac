package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/k8s"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// unifiedSession represents both a local session and a remote K8s pod
type unifiedSession struct {
	Name         string
	Status       string
	StartTime    time.Time
	LastActivity time.Time
	EndTime      time.Time
	Duration     time.Duration
	Location     string
	Cost         float64
	HasCost      bool
	Tokens       agent.TokenUsage
	Goal         string
}

func init() {
	rootCmd.AddCommand(psCmd)
	if psCmd.Flags().Lookup("costs") == nil {
		psCmd.Flags().BoolP("costs", "c", false, "Show token usage and cost information")
	}
	if psCmd.Flags().Lookup("sort") == nil {
		psCmd.Flags().String("sort", "time", "Sort sessions by 'cost', 'time', or 'name'")
	}
	if psCmd.Flags().Lookup("errors") == nil {
		psCmd.Flags().BoolP("errors", "e", false, "Show the first line of the error for failed sessions")
	}
	if psCmd.Flags().Lookup("remote") == nil {
		psCmd.Flags().Bool("remote", false, "Include remote Kubernetes pods in the list")
	}
	if psCmd.Flags().Lookup("status") == nil {
		psCmd.Flags().String("status", "", "Filter sessions by status (e.g., 'running', 'completed', 'error')")
	}
	if psCmd.Flags().Lookup("since") == nil {
		psCmd.Flags().String("since", "", "Filter sessions started after a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
	}
	if psCmd.Flags().Lookup("show-diff") == nil {
		psCmd.Flags().Bool("show-diff", false, "Show git diff for the most recent or specified session")
	}
	if psCmd.Flags().Lookup("session") == nil {
		psCmd.Flags().String("session", "", "Specify a session for --show-diff")
	}
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    `List all active and completed local sessions and, optionally, remote Kubernetes pods.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var allSessions []unifiedSession

		// --- Get Local Sessions ---
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		localSessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list local sessions: %w", err)
		}
		for _, s := range localSessions {
			us := unifiedSession{
				Name:      s.Name,
				Status:    s.Status,
				StartTime: s.StartTime,
				EndTime:   s.EndTime,
				Location:  "local",
			}
			// Calculate cost and tokens for local sessions
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				us.Cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				us.Tokens = agentState.TokenUsage
				us.HasCost = true
				us.LastActivity = agentState.LastActivity
				// Extract the goal from the first user message
				for _, msg := range agentState.History {
					if msg.Role == "user" {
						// Use the first line of the content as the goal
						firstLine := strings.Split(msg.Content, "\n")[0]
						us.Goal = strings.TrimSuffix(firstLine, ".")
						break
					}
				}
			}
			allSessions = append(allSessions, us)
		}

		// --- Get Remote Pods (if requested) ---
		showRemote, _ := cmd.Flags().GetBool("remote")
		if showRemote {
			k8sClient, err := k8s.NewClient()
			if err != nil {
				// Don't fail hard, just warn. Allows `ps` to work even if k8s is not configured.
				cmd.PrintErrf("Warning: Could not connect to Kubernetes: %v\n", err)
			} else {
				pods, err := k8sClient.ListPods(context.Background(), "app=recac-agent")
				if err != nil {
					return fmt.Errorf("failed to list Kubernetes pods: %w", err)
				}
				for _, pod := range pods {
					us := unifiedSession{
						Name:      pod.Labels["ticket"], // Assuming ticket label holds the session name
						Status:    string(pod.Status.Phase),
						StartTime: pod.CreationTimestamp.Time,
						Location:  "k8s",
					}
					// Cost calculation for pods is not supported yet
					allSessions = append(allSessions, us)
				}
			}
		}

		// --- Filter by Status ---
		statusFilter, _ := cmd.Flags().GetString("status")
		if statusFilter != "" {
			var filteredSessions []unifiedSession
			for _, s := range allSessions {
				if strings.EqualFold(s.Status, statusFilter) {
					filteredSessions = append(filteredSessions, s)
				}
			}
			allSessions = filteredSessions
		}

		// --- Filter by Time ---
		sinceFilter, _ := cmd.Flags().GetString("since")
		if sinceFilter != "" {
			var sinceTime time.Time
			var err error

			// Try parsing as a relative duration (e.g., "1h", "30m")
			duration, err := time.ParseDuration(sinceFilter)
			if err == nil {
				sinceTime = time.Now().Add(-duration)
			} else {
				// If not a duration, try parsing as an absolute timestamp
				// Supports RFC3339 ("2006-01-02T15:04:05Z07:00") and a simple date ("2006-01-02")
				layouts := []string{time.RFC3339, "2006-01-02"}
				parsed := false
				for _, layout := range layouts {
					t, err := time.Parse(layout, sinceFilter)
					if err == nil {
						sinceTime = t
						parsed = true
						break
					}
				}
				if !parsed {
					return fmt.Errorf("invalid 'since' value %q: must be a duration (e.g., '2h') or a timestamp (e.g., '2006-01-02')", sinceFilter)
				}
			}

			var filteredSessions []unifiedSession
			for _, s := range allSessions {
				if s.StartTime.After(sinceTime) {
					filteredSessions = append(filteredSessions, s)
				}
			}
			allSessions = filteredSessions
		}

		if len(allSessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		// --- Sort all sessions ---
		sortBy, _ := cmd.Flags().GetString("sort")
		sort.SliceStable(allSessions, func(i, j int) bool {
			switch sortBy {
			case "cost":
				if allSessions[i].HasCost && allSessions[j].HasCost {
					return allSessions[i].Cost > allSessions[j].Cost
				}
				return allSessions[i].HasCost // Ones with cost come first
			case "name":
				return allSessions[i].Name < allSessions[j].Name
			case "time":
				fallthrough
			default:
				return allSessions[i].StartTime.After(allSessions[j].StartTime)
			}
		})

		// --- Print Output ---
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		showCosts, _ := cmd.Flags().GetBool("costs")
		header := "NAME\tSTATUS\tLOCATION\tLAST USED\tGOAL"
		if showCosts {
			header += "\tPROMPT_TOKENS\tCOMPLETION_TOKENS\tTOTAL_TOKENS\tCOST"
		}
		fmt.Fprintln(w, header)

		for _, s := range allSessions {
			lastUsed := formatSince(s.LastActivity)
			if s.Location == "k8s" { // K8s pods don't have activity, use start time
				lastUsed = formatSince(s.StartTime)
			}

			// Truncate goal for better display
			goal := s.Goal
			if len(goal) > 60 {
				goal = goal[:57] + "..."
			}

			baseOutput := fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
				s.Name, s.Status, s.Location, lastUsed, goal)

			if showCosts {
				if s.HasCost {
					fmt.Fprintf(w, "%s\t%d\t%d\t%d\t$%.6f\n",
						baseOutput, s.Tokens.TotalPromptTokens, s.Tokens.TotalResponseTokens, s.Tokens.TotalTokens, s.Cost)
				} else {
					fmt.Fprintf(w, "%s\tN/A\tN/A\tN/A\tN/A\n", baseOutput)
				}
			} else {
				fmt.Fprintf(w, "%s\n", baseOutput)
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}

		// --- Handle --show-diff ---
		showDiff, _ := cmd.Flags().GetBool("show-diff")
		if showDiff {
			sessionName, _ := cmd.Flags().GetString("session")
			if sessionName == "" {
				// Find the most recent session if not specified
				if len(allSessions) > 0 {
					sessionName = allSessions[0].Name // Assumes default sort by time
				} else {
					return fmt.Errorf("no sessions available to diff")
				}
			}
			cmd.Println() // Add a newline for better formatting
			return handleSingleSessionDiff(cmd, sm, sessionName)
		}

		return nil
	},
}

func handleSingleSessionDiff(cmd *cobra.Command, sm ISessionManager, sessionName string) error {
	session, err := sm.LoadSession(sessionName)
	if err != nil {
		return fmt.Errorf("failed to load session %s: %w", sessionName, err)
	}

	if session.StartCommitSHA == "" {
		return fmt.Errorf("session '%s' does not have a start commit SHA recorded", sessionName)
	}

	endSHA, err := getSessionEndSHA(session)
	if err != nil {
		return err
	}

	gitClient := gitClientFactory()
	diff, err := gitClient.Diff(session.Workspace, session.StartCommitSHA, endSHA)
	if err != nil {
		return fmt.Errorf("failed to get git diff: %w", err)
	}

	cmd.Println(diff)
	return nil
}

// formatSince returns a human-readable string representing the time elapsed since t.
func formatSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}

	const (
		day  = 24 * time.Hour
		week = 7 * day
	)

	since := time.Since(t)
	if since < time.Minute {
		return fmt.Sprintf("%ds ago", int(since.Seconds()))
	}
	if since < time.Hour {
		return fmt.Sprintf("%dm ago", int(since.Minutes()))
	}
	if since < day {
		return fmt.Sprintf("%dh ago", int(since.Hours()))
	}
	if since < week {
		return fmt.Sprintf("%dd ago", int(since.Hours()/24))
	}
	// Fallback to absolute date for longer durations
	return t.Format("2006-01-02")
}
