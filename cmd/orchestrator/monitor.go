package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/model"
	"recac/internal/runner"
	"recac/internal/ui"
	"strings"
	"time"

	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Interactive session control center",
	Long:  `Launches a Terminal UI (TUI) to monitor and control active sessions managed by the orchestrator (local mode).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize SessionManager (Local Mode)
		sm, err := runner.NewSessionManager()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// Initialize Docker Client (Best effort)
		dockerCli, err := docker.NewClient("recac-orchestrator")
		if err != nil {
			// Log warning but continue?
			fmt.Printf("Warning: Failed to initialize Docker client: %v\n", err)
		} else {
			defer dockerCli.Close()
		}

		// Helper to adapt sessions
		getSessions := func() ([]model.UnifiedSession, error) {
			return getUnifiedSessions(sm)
		}

		callbacks := ui.ActionCallbacks{
			GetSessions: getSessions,
			Stop: func(name string) error {
				session, err := sm.LoadSession(name)
				if err != nil {
					return err
				}
				if session.Type == "orchestrated-docker" && session.ContainerID != "" && dockerCli != nil {
					// Use Docker Client to stop container
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := dockerCli.StopContainer(ctx, session.ContainerID); err != nil {
						// Continue to update status even if stop fails (might be already stopped)
						// But return error if it's not "not found"
					}
					session.Status = "stopped"
					session.EndTime = time.Now()
					return sm.SaveSession(session)
				}
				return sm.StopSession(name)
			},
			Pause: func(name string) error {
				session, err := sm.LoadSession(name)
				if err != nil {
					return err
				}
				if session.Type == "orchestrated-docker" {
					return fmt.Errorf("pausing docker sessions is not supported yet")
				}
				return sm.PauseSession(name)
			},
			Resume: func(name string) error {
				session, err := sm.LoadSession(name)
				if err != nil {
					return err
				}
				if session.Type == "orchestrated-docker" {
					return fmt.Errorf("resuming docker sessions is not supported yet")
				}
				return sm.ResumeSession(name)
			},
			GetLogs: func(name string) (string, error) {
				session, err := sm.LoadSession(name)
				if err != nil {
					return "", err
				}
				if session.Type == "orchestrated-docker" && session.ContainerID != "" && dockerCli != nil {
					// Fetch logs from Docker
					// Note: client.Exec returns output of a command, not logs.
					// We need ContainerLogs. internal/docker/client.go doesn't expose it yet.
					// So we fallback to session.LogFile if it exists.
					if session.LogFile != "" {
						return sm.GetSessionLogContent(name, 1000)
					}
					return "Logs not available for Docker session (LogFile not set and ContainerLogs not implemented)", nil
				}
				return sm.GetSessionLogContent(name, 1000)
			},
		}

		return ui.StartMonitorDashboard(callbacks)
	},
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

// loadAgentState is a helper to read and parse an agent state file.
var loadAgentState = func(filePath string) (*agent.State, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var state agent.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// getUnifiedSessions retrieves local sessions and maps them to UnifiedSession model.
func getUnifiedSessions(sm runner.ISessionManager) ([]model.UnifiedSession, error) {
	var allSessions []model.UnifiedSession

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
			Goal:      s.Goal, // Use the goal from session state if available
		}

		if s.Type == "orchestrated-docker" {
			us.Location = "docker"
		}

		// Calculate cost and tokens for local sessions
		if s.AgentStateFile != "" {
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				us.Cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				us.Tokens = agentState.TokenUsage
				us.HasCost = true
				us.LastActivity = agentState.LastActivity

				// If goal is missing in session state, try to get it from agent history
				if us.Goal == "" {
					for _, msg := range agentState.History {
						if msg.Role == "user" {
							firstLine := strings.Split(msg.Content, "\n")[0]
							us.Goal = strings.TrimSuffix(firstLine, ".")
							break
						}
					}
				}
			}
		}

		us.CPU = "N/A"
		us.Memory = "N/A"
		// Get CPU and Memory usage for local running processes (not docker)
		if s.Status == "running" && s.PID > 0 && s.Type != "orchestrated-docker" {
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

		allSessions = append(allSessions, us)
	}

	return allSessions, nil
}
