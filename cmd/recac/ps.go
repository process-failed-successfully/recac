package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"recac/internal/agent"
	"recac/internal/orchestrator"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// unifiedSession represents both a local session and a remote K8s pod
type unifiedSession struct {
	Name      string
	Status    string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Location  string
	Cost      float64
	HasCost   bool
	Tokens    agent.TokenUsage
	Tags      []string
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
	if psCmd.Flags().Lookup("tag") == nil {
		psCmd.Flags().String("tag", "", "Filter sessions by tag")
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
				Tags:      s.Tags,
			}
			// Calculate cost and tokens for local sessions
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				us.Cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				us.Tokens = agentState.TokenUsage
				us.HasCost = true
			}
			allSessions = append(allSessions, us)
		}

		// --- Get Remote Pods (if requested) ---
		showRemote, _ := cmd.Flags().GetBool("remote")
		if showRemote {
			// Using a null logger as we don't want spawner logs in `ps` output
			nullLogger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
			spawner, err := orchestrator.NewK8sSpawner(nullLogger, "", "", "", "", "")
			if err != nil {
				// Don't fail hard, just warn. Allows `ps` to work even if k8s is not configured.
				cmd.PrintErrf("Warning: Could not connect to Kubernetes: %v\n", err)
			} else {
				pods, err := spawner.Client.CoreV1().Pods(spawner.Namespace).List(context.Background(), metav1.ListOptions{
					LabelSelector: "app=recac-agent",
				})
				if err != nil {
					return fmt.Errorf("failed to list Kubernetes pods: %w", err)
				}
				for _, pod := range pods.Items {
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

		// --- Filter by Tag ---
		tagFilter, _ := cmd.Flags().GetString("tag")
		if tagFilter != "" {
			var filteredSessions []unifiedSession
			for _, s := range allSessions {
				for _, t := range s.Tags {
					if t == tagFilter {
						filteredSessions = append(filteredSessions, s)
						break
					}
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
		header := "NAME\tSTATUS\tLOCATION\tSTARTED\tDURATION"
		if showCosts {
			header += "\tPROMPT_TOKENS\tCOMPLETION_TOKENS\tTOTAL_TOKENS\tCOST"
		}
		fmt.Fprintln(w, header)

		for _, s := range allSessions {
			started := s.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if s.EndTime.IsZero() {
				duration = time.Since(s.StartTime).Round(time.Second).String()
			} else {
				duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
			}

			baseOutput := fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
				s.Name, s.Status, s.Location, started, duration)

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

		return w.Flush()
	},
}
