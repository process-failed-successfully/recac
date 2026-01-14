package main

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/runner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(topCmd)
}

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Monitor running recac sessions in real-time",
	Long: `Provides a real-time, dynamic view of all running recac agent sessions,
displaying CPU usage, memory consumption, and other vital stats.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(newTopModel())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

// --- TUI MODEL ---

type processMetrics struct {
	PID        int32
	CPUPercent float64
	MemPercent float32
	MemRSS     uint64 // Resident Set Size
}

type topModel struct {
	sessions []*runner.SessionState
	metrics  map[string]processMetrics
	err      error
}

type topTickMsg time.Time
type sessionsRefreshedMsg struct {
	sessions []*runner.SessionState
	metrics  map[string]processMetrics
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	headerStyle = lipgloss.NewStyle().Bold(true)
)

func newTopModel() topModel {
	return topModel{
		metrics: make(map[string]processMetrics),
	}
}

func (m topModel) Init() tea.Cmd {
	return tea.Batch(refreshTopCmd(), tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return topTickMsg(t)
	}))
}

func (m topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case topTickMsg:
		return m, refreshTopCmd()
	case sessionsRefreshedMsg:
		m.sessions = msg.sessions
		m.metrics = msg.metrics
		return m, nil
	case error:
		m.err = msg
		return m, nil
	}
	return m, nil
}

func (m topModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\n(press 'q' to quit)", m.err)
	}

	var s strings.Builder
	s.WriteString(titleStyle.Render("🚀 RECAC Session Monitor") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'q' to quit)\n", time.Now().Format(time.RFC1123)))

	if len(m.sessions) == 0 {
		s.WriteString("\nNo running sessions found.")
		return s.String()
	}

	w := tabwriter.NewWriter(&s, 0, 0, 3, ' ', 0)
	header := fmt.Sprintf("NAME\t%s\t%s\t%s\t%s\t%s",
		headerStyle.Render("PID"),
		headerStyle.Render("STATUS"),
		headerStyle.Render("CPU %"),
		headerStyle.Render("MEM %"),
		headerStyle.Render("DURATION"))
	fmt.Fprintln(w, header)

	for _, session := range m.sessions {
		metric := m.metrics[session.Name]
		duration := time.Since(session.StartTime).Round(time.Second)

		fmt.Fprintf(w, "%s\t%d\t%s\t%.2f\t%.2f\t%s\n",
			session.Name,
			metric.PID,
			session.Status,
			metric.CPUPercent,
			metric.MemPercent,
			duration,
		)
	}

	w.Flush()
	return s.String()
}

func refreshTopCmd() tea.Cmd {
	return func() tea.Msg {
		sm, err := sessionManagerFactory()
		if err != nil {
			return err
		}

		allSessions, err := sm.ListSessions()
		if err != nil {
			return err
		}

		runningSessions := make([]*runner.SessionState, 0)
		for _, s := range allSessions {
			if s.Status == "running" {
				runningSessions = append(runningSessions, s)
			}
		}

		metrics := make(map[string]processMetrics)
		for _, s := range runningSessions {
			if s.PID == 0 {
				continue
			}

			p, err := process.NewProcess(int32(s.PID))
			// Check if process exists
			if err != nil {
				// Process might have just finished, not a critical error
				continue
			}

			cpu, err := p.CPUPercent()
			if err != nil {
				continue
			}

			mem, err := p.MemoryPercent()
			if err != nil {
				continue
			}

			metrics[s.Name] = processMetrics{
				PID:        int32(s.PID),
				CPUPercent: cpu,
				MemPercent: mem,
			}
		}

		return sessionsRefreshedMsg{sessions: runningSessions, metrics: metrics}
	}
}
