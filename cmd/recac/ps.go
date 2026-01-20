package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/k8s"
	"recac/internal/model"
	"recac/internal/ui"
	"recac/internal/utils"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

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
	if psCmd.Flags().Lookup("stale") == nil {
		psCmd.Flags().String("stale", "", "Filter sessions that have been inactive for a given duration (e.g., '7d', '24h')")
	}
	if psCmd.Flags().Lookup("watch") == nil {
		psCmd.Flags().BoolP("watch", "w", false, "Enter watch mode with real-time updates")
	}
	if psCmd.Flags().Lookup("logs") == nil {
		psCmd.Flags().Int("logs", 0, "Show the last N lines of logs for each session")
	}
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"list"},
	Short:   "List sessions",
	Long:    `List all active and completed local sessions and, optionally, remote Kubernetes pods.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Get Flags ---
		showCosts, _ := cmd.Flags().GetBool("costs")
		sortBy, _ := cmd.Flags().GetString("sort")
		showDiff, _ := cmd.Flags().GetBool("show-diff")
		sessionName, _ := cmd.Flags().GetString("session")
		watch, _ := cmd.Flags().GetBool("watch")
		logLines, _ := cmd.Flags().GetInt("logs")

		filters := model.PsFilters{
			Status:   cmd.Flag("status").Value.String(),
			Since:    cmd.Flag("since").Value.String(),
			Stale:    cmd.Flag("stale").Value.String(),
			Remote:   cmd.Flag("remote").Value.String() == "true",
			LogLines: logLines,
		}

		// --- Handle Watch Mode ---
		if watch {
			// Dependency injection: Provide the UI with a function to get sessions
			ui.GetSessions = func() ([]model.UnifiedSession, error) {
				// We pass the *current* command instance to getUnifiedSessions
				return getUnifiedSessions(cmd, filters)
			}
			return ui.StartPsDashboard()
		}

		// --- Get Sessions ---
		allSessions, err := getUnifiedSessions(cmd, filters)
		if err != nil {
			return err
		}

		if len(allSessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		// --- Sort all sessions ---
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
		header := "NAME\tSTATUS\tCPU\tMEM\tLOCATION\tLAST USED\tGOAL"
		if showCosts {
			header += "\tPROMPT_TOKENS\tCOMPLETION_TOKENS\tTOTAL_TOKENS\tCOST"
		}
		fmt.Fprintln(w, header)

		for _, s := range allSessions {
			lastUsed := utils.FormatSince(s.LastActivity)
			if s.Location == "k8s" { // K8s pods don't have activity, use start time
				lastUsed = utils.FormatSince(s.StartTime)
			}

			// Truncate goal for better display
			goal := s.Goal
			if len(goal) > 60 {
				goal = goal[:57] + "..."
			}

			baseOutput := fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s",
				s.Name, s.Status, s.CPU, s.Memory, s.Location, lastUsed, goal)

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

			// --- Show Logs ---
			if filters.LogLines > 0 && s.Logs != "" {
				// Indent logs for readability
				logLines := strings.Split(s.Logs, "\n")
				for _, line := range logLines {
					if line != "" {
						fmt.Fprintf(w, "  └ %s\n", line)
					}
				}
			}
		}

		if err := w.Flush(); err != nil {
			return err
		}

		// --- Handle --show-diff ---
		if showDiff {
			if sessionName == "" {
				// Find the most recent session if not specified
				if len(allSessions) > 0 {
					sessionName = allSessions[0].Name // Assumes default sort by time
				} else {
					return fmt.Errorf("no sessions available to diff")
				}
			}
			cmd.Println() // Add a newline for better formatting
			sm, err := sessionManagerFactory()
			if err != nil {
				return fmt.Errorf("failed to create session manager for diff: %w", err)
			}
			return handleSingleSessionDiff(cmd, sm, sessionName)
		}

		return nil
	},
}

// getUnifiedSessions retrieves and filters both local and remote sessions.
func getUnifiedSessions(cmd *cobra.Command, filters model.PsFilters) ([]model.UnifiedSession, error) {
	var allSessions []model.UnifiedSession

	// --- Get Local Sessions ---
	sm, err := sessionManagerFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}
	localSessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list local sessions: %w", err)
	}
	for _, s := range localSessions {
		us := model.UnifiedSession{
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
					firstLine := strings.Split(msg.Content, "\n")[0]
					us.Goal = strings.TrimSuffix(firstLine, ".")
					break
				}
			}
		}

		us.CPU = "N/A"
		us.Memory = "N/A"
		// Get CPU and Memory usage for local running sessions
		if s.Status == "running" && s.PID > 0 {
			p, err := process.NewProcess(int32(s.PID))
			if err == nil {
				cpuPercent, err := p.CPUPercent()
				if err == nil {
					us.CPU = fmt.Sprintf("%.1f%%", cpuPercent)
				}
				memInfo, err := p.MemoryInfo()
				if err == nil {
					us.Memory = fmt.Sprintf("%dMB", memInfo.RSS/1024/1024)
				}
			}
		}

		// --- Get Logs if requested ---
		if filters.LogLines > 0 {
			logs, err := sm.GetSessionLogContent(s.Name, filters.LogLines)
			if err == nil {
				us.Logs = logs
			}
		}
		allSessions = append(allSessions, us)
	}

	// --- Get Remote Pods (if requested) ---
	if filters.Remote {
		k8sClient, err := k8s.NewClient()
		if err != nil {
			cmd.PrintErrf("Warning: Could not connect to Kubernetes: %v\n", err)
		} else {
			pods, err := k8sClient.ListPods(context.Background(), "app=recac-agent")
			if err != nil {
				return nil, fmt.Errorf("failed to list Kubernetes pods: %w", err)
			}
			for _, pod := range pods {
				us := model.UnifiedSession{
					Name:      pod.Labels["ticket"],
					Status:    string(pod.Status.Phase),
					StartTime: pod.CreationTimestamp.Time,
					Location:  "k8s",
				}
				allSessions = append(allSessions, us)
			}
		}
	}

	// --- Filter by Status ---
	if filters.Status != "" {
		var filteredSessions []model.UnifiedSession
		for _, s := range allSessions {
			if strings.EqualFold(s.Status, filters.Status) {
				filteredSessions = append(filteredSessions, s)
			}
		}
		allSessions = filteredSessions
	}

	// --- Filter by Stale ---
	if filters.Stale != "" {
		duration, err := utils.ParseStaleDuration(filters.Stale)
		if err != nil {
			return nil, fmt.Errorf("invalid 'stale' value %q: %w", filters.Stale, err)
		}
		staleTime := time.Now().Add(-duration)
		var filteredSessions []model.UnifiedSession
		for _, s := range allSessions {
			activityTime := s.LastActivity
			if s.Location == "k8s" {
				activityTime = s.StartTime
			}
			if !activityTime.IsZero() && activityTime.Before(staleTime) {
				filteredSessions = append(filteredSessions, s)
			}
		}
		allSessions = filteredSessions
	}

	// --- Filter by Time ---
	if filters.Since != "" {
		var sinceTime time.Time
		var err error
		duration, err := time.ParseDuration(filters.Since)
		if err == nil {
			sinceTime = time.Now().Add(-duration)
		} else {
			parsed := false
			// Try RFC3339 first (absolute)
			if t, err := time.Parse(time.RFC3339, filters.Since); err == nil {
				sinceTime = t
				parsed = true
			} else {
				// Try date-only in Local time
				if t, err := time.ParseInLocation("2006-01-02", filters.Since, time.Local); err == nil {
					sinceTime = t
					parsed = true
				}
			}

			if !parsed {
				return nil, fmt.Errorf("invalid 'since' value %q: must be a duration or timestamp", filters.Since)
			}
		}
		var filteredSessions []model.UnifiedSession
		for _, s := range allSessions {
			if s.StartTime.After(sinceTime) {
				filteredSessions = append(filteredSessions, s)
			}
		}
		allSessions = filteredSessions
	}

	return allSessions, nil
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
