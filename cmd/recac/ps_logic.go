package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/k8s"
	"recac/internal/model"
	"sort"
	"strings"
	"time"

	"github.com/spf13/pflag"
)

// GetUnifiedSessions fetches, merges, filters, and sorts sessions from local and remote sources.
var GetUnifiedSessions = func(flags *pflag.FlagSet) ([]model.UnifiedSession, []string, error) {
	var allSessions []model.UnifiedSession
	var warnings []string

	// --- Get Local Sessions ---
	sm, err := sessionManagerFactory()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session manager: %w", err)
	}
	localSessions, err := sm.ListSessions()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list local sessions: %w", err)
	}
	for _, s := range localSessions {
		us := model.UnifiedSession{
			Name:      s.Name,
			Status:    s.Status,
			StartTime: s.StartTime,
			EndTime:   s.EndTime,
			Location:  "local",
		}
		agentState, err := loadAgentState(s.AgentStateFile)
		if err == nil {
			us.Cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			us.Tokens = agentState.TokenUsage
			us.HasCost = true
		}
		allSessions = append(allSessions, us)
	}

	// --- Get Remote Pods (if requested) ---
	showRemote, _ := flags.GetBool("remote")
	if showRemote {
		k8sClient, err := k8s.NewClient()
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Could not connect to Kubernetes: %v", err))
		} else {
			pods, err := k8sClient.ListPods(context.Background(), "app=recac-agent")
			if err != nil {
				return nil, warnings, fmt.Errorf("failed to list Kubernetes pods: %w", err)
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
	statusFilter, _ := flags.GetString("status")
	if statusFilter != "" {
		var filteredSessions []model.UnifiedSession
		for _, s := range allSessions {
			if strings.EqualFold(s.Status, statusFilter) {
				filteredSessions = append(filteredSessions, s)
			}
		}
		allSessions = filteredSessions
	}

	// --- Filter by Time ---
	sinceFilter, _ := flags.GetString("since")
	if sinceFilter != "" {
		sinceTime, err := parseSince(sinceFilter)
		if err != nil {
			return nil, warnings, err
		}

		var filteredSessions []model.UnifiedSession
		for _, s := range allSessions {
			if s.StartTime.After(sinceTime) {
				filteredSessions = append(filteredSessions, s)
			}
		}
		allSessions = filteredSessions
	}

	// --- Sort all sessions ---
	sortBy, _ := flags.GetString("sort")
	sort.SliceStable(allSessions, func(i, j int) bool {
		switch sortBy {
		case "cost":
			if allSessions[i].HasCost && allSessions[j].HasCost {
				return allSessions[i].Cost > allSessions[j].Cost
			}
			return allSessions[i].HasCost
		case "name":
			return allSessions[i].Name < allSessions[j].Name
		case "time":
			fallthrough
		default:
			return allSessions[i].StartTime.After(allSessions[j].StartTime)
		}
	})

	return allSessions, warnings, nil
}

func parseSince(sinceFilter string) (time.Time, error) {
	duration, err := time.ParseDuration(sinceFilter)
	if err == nil {
		return time.Now().Add(-duration), nil
	}

	layouts := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, sinceFilter)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid 'since' value %q: must be a duration (e.g., '2h') or a timestamp (e.g., '2006-01-02')", sinceFilter)
}
