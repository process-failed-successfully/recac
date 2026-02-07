package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/k8s"
	"recac/internal/model"
	"recac/internal/runner"
	"recac/internal/ui"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

// Interfaces for dependency injection
type SessionLister interface {
	ListSessions() ([]*runner.SessionState, error)
	GetSessionLogContent(name string, lines int) (string, error)
}

type PodLister interface {
	ListPods(ctx context.Context, labelSelector string) ([]corev1.Pod, error)
}

// Factories for dependency injection
var (
	sessionManagerFactory = func() (SessionLister, error) {
		return runner.NewSessionManager()
	}
	k8sClientFactory = func() (PodLister, error) {
		return k8s.NewClient()
	}
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Monitor active agents and sessions",
	Long:  `Display a real-time dashboard of all active coding agents, both local (Docker) and remote (Kubernetes).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showCosts, _ := cmd.Flags().GetBool("costs")
		sortBy, _ := cmd.Flags().GetString("sort")
		remote, _ := cmd.Flags().GetBool("remote")
		logs, _ := cmd.Flags().GetInt("logs")

		opts := DashboardOptions{
			ShowCosts: showCosts,
			SortBy:    sortBy,
			Remote:    remote,
			LogLines:  logs,
		}

		// Set the callback for the UI
		ui.GetSessions = func() ([]model.UnifiedSession, error) {
			return getUnifiedSessions(opts)
		}

		return ui.StartPsDashboard(showCosts, sortBy)
	},
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	dashboardCmd.Flags().BoolP("costs", "c", false, "Show token usage and cost information")
	dashboardCmd.Flags().String("sort", "time", "Sort sessions by 'cost', 'time', or 'name'")
	dashboardCmd.Flags().Bool("remote", false, "Include remote Kubernetes pods in the list")
	dashboardCmd.Flags().Int("logs", 0, "Show the last N lines of logs for each session")
}

type DashboardOptions struct {
	ShowCosts bool
	SortBy    string
	Remote    bool
	LogLines  int
}

func getUnifiedSessions(opts DashboardOptions) ([]model.UnifiedSession, error) {
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
		agentState, err := agent.LoadState(s.AgentStateFile)
		if err == nil && agentState != nil {
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
		if opts.LogLines > 0 {
			logs, err := sm.GetSessionLogContent(s.Name, opts.LogLines)
			if err == nil {
				us.Logs = logs
			}
		}
		allSessions = append(allSessions, us)
	}

	// --- Get Remote Pods (if requested) ---
	if opts.Remote {
		k8sClient, err := k8sClientFactory()
		if err != nil {
			// Just warn if K8s is not available but requested
			fmt.Printf("Warning: Could not connect to Kubernetes: %v\n", err)
		} else {
			pods, err := k8sClient.ListPods(context.Background(), "app=recac-agent")
			if err != nil {
				return nil, fmt.Errorf("failed to list Kubernetes pods: %w", err)
			}
			for _, pod := range pods {
				// Try to parse start time
				startTime := pod.CreationTimestamp.Time

				// Goal is often in annotations or labels, but for now we might leave it empty or use a label
				goal := pod.Annotations["recac.goal"]
				if goal == "" {
					goal = pod.Labels["ticket"]
				}

				us := model.UnifiedSession{
					Name:      pod.Name,
					Status:    string(pod.Status.Phase),
					StartTime: startTime,
					Location:  "k8s",
					Goal:      goal,
				}
				allSessions = append(allSessions, us)
			}
		}
	}

	// Sort by start time (newest first) by default
	sort.Slice(allSessions, func(i, j int) bool {
		return allSessions[i].StartTime.After(allSessions[j].StartTime)
	})

	return allSessions, nil
}
