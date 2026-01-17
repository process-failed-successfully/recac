package main

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/model"
	"recac/internal/ui"
	"sort"

	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

var dashboardCmd = &cobra.Command{
	Use:   "dashboard [session-name]",
	Short: "Interactive live dashboard for a specific session",
	Long:  `Launches a TUI dashboard for a specific session, showing live logs, agent thoughts, resources, and status. If no session name is provided, it attempts to connect to the most recent running session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var sessionName string
		if len(args) > 0 {
			sessionName = args[0]
		} else {
			// Find most recent running session
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			// Sort by start time desc
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].StartTime.After(sessions[j].StartTime)
			})

			if len(sessions) == 0 {
				return fmt.Errorf("no sessions found")
			}
			sessionName = sessions[0].Name
			fmt.Printf("No session specified. Connecting to most recent session: %s\n", sessionName)
		}

		// Inject dependencies into UI
		ui.GetSessionDetail = func(name string) (*model.UnifiedSession, error) {
			s, err := sm.LoadSession(name)
			if err != nil {
				return nil, err
			}

			// Map runner.SessionState to model.UnifiedSession
			us := &model.UnifiedSession{
				Name:      s.Name,
				Status:    s.Status,
				StartTime: s.StartTime,
				EndTime:   s.EndTime,
				Location:  "local",
				Goal:      s.Goal,
			}

			// Get Resources
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
			if us.CPU == "" { us.CPU = "N/A" }
			if us.Memory == "" { us.Memory = "N/A" }

			// Get Cost/Tokens
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				us.Tokens = agentState.TokenUsage
				us.Cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				us.HasCost = true
			}

			return us, nil
		}

		ui.GetSessionLogs = func(name string) (string, error) {
			// Get last 100 lines
			return sm.GetSessionLogContent(name, 100)
		}

		ui.GetAgentState = func(name string) (*agent.State, error) {
			s, err := sm.LoadSession(name)
			if err != nil {
				return nil, err
			}
			return loadAgentState(s.AgentStateFile)
		}

		return ui.StartSessionDashboard(sessionName)
	},
}
