package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

// Events
type PollStartMsg struct{}
type PollEndMsg struct {
	Count int
	Err   error
}
type SpawnStartMsg struct {
	Item orchestrator.WorkItem
}
type SpawnEndMsg struct {
	Item orchestrator.WorkItem
	Err  error
}

type OrchestratorDashboardModel struct {
	events chan tea.Msg

	lastPollTime time.Time
	pollStatus   string

	activeSpawns map[string]time.Time // ID -> StartTime
	history      []string             // Simple log of spawn events

	viewport viewport.Model
	ready    bool
}

func NewOrchestratorDashboard() *OrchestratorDashboardModel {
	return &OrchestratorDashboardModel{
		events:       make(chan tea.Msg, 100),
		activeSpawns: make(map[string]time.Time),
		pollStatus:   "Waiting...",
	}
}

// Observer implementation
func (m *OrchestratorDashboardModel) OnPollStart() {
	select {
	case m.events <- PollStartMsg{}:
	default:
	}
}
func (m *OrchestratorDashboardModel) OnPollEnd(count int, err error) {
	select {
	case m.events <- PollEndMsg{Count: count, Err: err}:
	default:
	}
}
func (m *OrchestratorDashboardModel) OnSpawnStart(item orchestrator.WorkItem) {
	select {
	case m.events <- SpawnStartMsg{Item: item}:
	default:
	}
}
func (m *OrchestratorDashboardModel) OnSpawnEnd(item orchestrator.WorkItem, err error) {
	select {
	case m.events <- SpawnEndMsg{Item: item, Err: err}:
	default:
	}
}

// TEA
func (m *OrchestratorDashboardModel) Init() tea.Cmd {
	return waitForEvent(m.events)
}

func waitForEvent(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m *OrchestratorDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		headerHeight := 8 // Approx
		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight
		}

	// Events
	case PollStartMsg:
		m.pollStatus = "Polling..."
		cmds = append(cmds, waitForEvent(m.events))
	case PollEndMsg:
		m.lastPollTime = time.Now()
		if msg.Err != nil {
			m.pollStatus = fmt.Sprintf("Error: %v", msg.Err)
			m.addHistory(fmt.Sprintf("[%s] Poll Error: %v", timeFormat(), msg.Err))
		} else {
			m.pollStatus = fmt.Sprintf("Success (%d items found)", msg.Count)
			if msg.Count > 0 {
				m.addHistory(fmt.Sprintf("[%s] Poll Found %d items", timeFormat(), msg.Count))
			}
		}
		cmds = append(cmds, waitForEvent(m.events))
	case SpawnStartMsg:
		m.activeSpawns[msg.Item.ID] = time.Now()
		m.addHistory(fmt.Sprintf("[%s] Spawning %s", timeFormat(), msg.Item.ID))
		cmds = append(cmds, waitForEvent(m.events))
	case SpawnEndMsg:
		delete(m.activeSpawns, msg.Item.ID)
		status := "Started"
		if msg.Err != nil {
			status = fmt.Sprintf("Failed: %v", msg.Err)
		}
		m.addHistory(fmt.Sprintf("[%s] Spawned %s: %s", timeFormat(), msg.Item.ID, status))
		cmds = append(cmds, waitForEvent(m.events))
	}

	if m.ready {
		m.viewport.SetContent(m.renderHistory())
		// Auto scroll to bottom
		m.viewport.GotoBottom()
	}

	return m, tea.Batch(cmds...)
}

func (m *OrchestratorDashboardModel) addHistory(entry string) {
	m.history = append(m.history, entry)
	// Keep history bounded?
	if len(m.history) > 100 {
		m.history = m.history[1:]
	}
}

func (m *OrchestratorDashboardModel) renderHistory() string {
	return strings.Join(m.history, "\n")
}

func timeFormat() string {
	return time.Now().Format(time.TimeOnly)
}

func (m *OrchestratorDashboardModel) View() string {
	if !m.ready {
		return "Initializing TUI..."
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	s := titleStyle.Render("Orchestrator Dashboard") + "\n"

	pollTime := "Never"
	if !m.lastPollTime.IsZero() {
		pollTime = m.lastPollTime.Format(time.TimeOnly)
	}
	s += fmt.Sprintf("Last Poll: %s (%s)\n", pollTime, statusStyle.Render(m.pollStatus))

	s += "\nActive Spawns:\n"
	if len(m.activeSpawns) == 0 {
		s += "  (None)\n"
	} else {
		// Sort for stability
		var ids []string
		for id := range m.activeSpawns {
			ids = append(ids, id)
		}
		sort.Strings(ids)

		for _, id := range ids {
			start := m.activeSpawns[id]
			s += fmt.Sprintf("  - %s (%s)\n", id, time.Since(start).Round(time.Second))
		}
	}

	s += "\nHistory:\n"
	s += m.viewport.View()

	s += "\nPress q to quit."
	return s
}
