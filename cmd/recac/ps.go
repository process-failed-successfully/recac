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
	EndTime      time.Time
	LastActivity time.Time
	Duration     time.Duration
	Location     string
	Goal         string
	Cost         float64
	HasCost      bool
	Tokens       agent.TokenUsage
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
			// Get additional info from agent state
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				// Cost
				us.Cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				us.Tokens = agentState.TokenUsage
				us.HasCost = true
				// Last Activity
				us.LastActivity = agentState.UpdatedAt
				// Goal
				for _, msg := range agentState.History {
					if msg.Role == "user" {
						us.Goal = msg.Content
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
		header := "NAME\tGOAL\tSTATUS\tLOCATION\tAGE\tLAST ACTIVITY"
		if showCosts {
			header += "\tCOST"
		}
		fmt.Fprintln(w, header)

		for _, s := range allSessions {
			age := formatSince(s.StartTime)
			lastActivity := "N/A"
			if !s.LastActivity.IsZero() {
				lastActivity = formatSince(s.LastActivity)
			}

			// Truncate goal for display
			goal := s.Goal
			if len(goal) > 50 {
				goal = goal[:47] + "..."
			}

			baseOutput := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s",
				s.Name, goal, s.Status, s.Location, age, lastActivity)

			if showCosts {
				if s.HasCost {
					fmt.Fprintf(w, "%s\t$%.6f\n", baseOutput, s.Cost)
				} else {
					fmt.Fprintf(w, "%s\tN/A\n", baseOutput)
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
		return "N/A"
	}

	since := time.Since(t)

	if since.Hours() >= 24*30 {
		months := int(since.Hours() / (24 * 30))
		return fmt.Sprintf("%dmo ago", months)
	}
	if since.Hours() >= 24 {
		days := int(since.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
	if since.Minutes() >= 60 {
		hours := int(since.Minutes() / 60)
		return fmt.Sprintf("%dh ago", hours)
	}
	if since.Seconds() >= 60 {
		minutes := int(since.Seconds() / 60)
		return fmt.Sprintf("%dm ago", minutes)
	}
	return fmt.Sprintf("%ds ago", int(since.Seconds()))
}
