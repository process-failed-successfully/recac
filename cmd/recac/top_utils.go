package main

import (
	"fmt"
	"recac/internal/model"
	"strings"

	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

// getRunningSessions retrieves and filters only running local sessions.
func getRunningSessions(cmd *cobra.Command) ([]model.UnifiedSession, error) {
	var runningSessions []model.UnifiedSession

	sm, err := sessionManagerFactory()
	if err != nil {
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}
	localSessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("failed to list local sessions: %w", err)
	}

	for _, s := range localSessions {
		if s.Status != "running" {
			continue
		}

		us := model.UnifiedSession{
			Name:      s.Name,
			Status:    s.Status,
			StartTime: s.StartTime,
			Location:  "local",
		}

		agentState, err := loadAgentState(s.AgentStateFile)
		if err == nil {
			us.LastActivity = agentState.LastActivity
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
		if s.PID > 0 {
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
		runningSessions = append(runningSessions, us)
	}

	return runningSessions, nil
}
