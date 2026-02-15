package ui

import (
	"fmt"
	"recac/internal/orchestrator"
	"recac/internal/runner"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// OrchestratorDashboardModel implements the TUI for the orchestrator
type OrchestratorDashboardModel struct {
	program        *tea.Program
	sessionManager orchestrator.ISessionManager

	// State
	polling      bool
	lastPollTime time.Time
	lastPollErr  error
	pollCount    int

	events       []string
	activeSpawns map[string]orchestrator.WorkItem // Key: Item ID
	spawnsMu     sync.Mutex

	// UI Components
	table    table.Model
	viewport viewport.Model
	width    int
	height   int
}

// Ensure interface implementation
var _ orchestrator.Observer = (*OrchestratorDashboardModel)(nil)

// Messages
type pollStartMsg struct{}
type pollEndMsg struct {
	items []orchestrator.WorkItem
	err   error
}
type spawnStartMsg struct{ item orchestrator.WorkItem }
type spawnEndMsg struct {
	item orchestrator.WorkItem
	err  error
}
type orchTickMsg time.Time

// NewOrchestratorDashboardModel creates a new dashboard
func NewOrchestratorDashboardModel(sm orchestrator.ISessionManager) *OrchestratorDashboardModel {
	columns := []table.Column{
		{Title: "ID", Width: 15},
		{Title: "STATUS", Width: 10},
		{Title: "SUMMARY", Width: 40},
		{Title: "DURATION", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &OrchestratorDashboardModel{
		sessionManager: sm,
		activeSpawns:   make(map[string]orchestrator.WorkItem),
		events:         []string{},
		table:          t,
		viewport:       viewport.New(0, 0),
	}
}

// SetProgram sets the tea program reference for sending messages from background goroutines
func (m *OrchestratorDashboardModel) SetProgram(p *tea.Program) {
	m.program = p
}

// Observer Implementation
func (m *OrchestratorDashboardModel) OnPollStart() {
	if m.program != nil {
		m.program.Send(pollStartMsg{})
	}
}

func (m *OrchestratorDashboardModel) OnPollEnd(items []orchestrator.WorkItem, err error) {
	if m.program != nil {
		m.program.Send(pollEndMsg{items: items, err: err})
	}
}

func (m *OrchestratorDashboardModel) OnSpawnStart(item orchestrator.WorkItem) {
	if m.program != nil {
		m.program.Send(spawnStartMsg{item: item})
	}
}

func (m *OrchestratorDashboardModel) OnSpawnEnd(item orchestrator.WorkItem, err error) {
	if m.program != nil {
		m.program.Send(spawnEndMsg{item: item, err: err})
	}
}

// Bubble Tea Methods

func (m *OrchestratorDashboardModel) Init() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return orchTickMsg(t)
	})
}

func (m *OrchestratorDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
		m.table, cmd = m.table.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width)
		m.viewport.Width = m.width

		// Calculate available height for logs
		// Header (4) + Table (12) + Footer (2) = ~18 lines
		logHeight := m.height - 20
		if logHeight < 5 {
			logHeight = 5
		}
		m.viewport.Height = logHeight

	case pollStartMsg:
		m.polling = true
		m.addEvent("Polling for work...")

	case pollEndMsg:
		m.polling = false
		m.lastPollTime = time.Now()
		m.lastPollErr = msg.err
		if msg.err != nil {
			m.addEvent(fmt.Sprintf("Poll failed: %v", msg.err))
		} else {
			count := len(msg.items)
			if count > 0 {
				m.addEvent(fmt.Sprintf("Poll found %d items", count))
			}
		}
		m.pollCount++

	case spawnStartMsg:
		m.spawnsMu.Lock()
		m.activeSpawns[msg.item.ID] = msg.item
		m.spawnsMu.Unlock()
		m.addEvent(fmt.Sprintf("Spawning agent for %s", msg.item.ID))
		m.updateTable()

	case spawnEndMsg:
		m.spawnsMu.Lock()
		delete(m.activeSpawns, msg.item.ID)
		m.spawnsMu.Unlock()
		if msg.err != nil {
			m.addEvent(fmt.Sprintf("Spawn failed for %s: %v", msg.item.ID, msg.err))
		} else {
			m.addEvent(fmt.Sprintf("Spawned agent for %s", msg.item.ID))
		}
		m.updateTable()

	case orchTickMsg:
		// Refresh sessions if we have sessionManager
		// Also redraw table
		m.updateTable()
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return orchTickMsg(t)
		})
	}

	return m, cmd
}

func (m *OrchestratorDashboardModel) addEvent(msg string) {
	ts := time.Now().Format("15:04:05")
	m.events = append([]string{fmt.Sprintf("[%s] %s", ts, msg)}, m.events...)
	if len(m.events) > 100 {
		m.events = m.events[:100]
	}
	m.viewport.SetContent(strings.Join(m.events, "\n"))
}

func (m *OrchestratorDashboardModel) updateTable() {
	var rows []table.Row

	// Add active spawns (in-flight spawns)
	m.spawnsMu.Lock()
	for _, item := range m.activeSpawns {
		rows = append(rows, table.Row{
			item.ID,
			"SPAWNING",
			truncate(item.Summary, 40),
			"-",
		})
	}
	m.spawnsMu.Unlock()

	// Add active sessions from SessionManager
	if m.sessionManager != nil {
		if sm, ok := m.sessionManager.(interface {
			ListSessions() ([]*runner.SessionState, error)
		}); ok {
			sessions, err := sm.ListSessions()
			if err == nil {
				for _, s := range sessions {
					if s.Status == "running" {
						rows = append(rows, table.Row{
							s.Name,
							strings.ToUpper(s.Status),
							truncate(s.Goal, 40),
							time.Since(s.StartTime).Round(time.Second).String(),
						})
					}
				}
			}
		}
	}

	m.table.SetRows(rows)
}

func truncate(s string, l int) string {
	if len(s) > l {
		return s[:l-3] + "..."
	}
	return s
}

var (
	orchTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	orchHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

func (m *OrchestratorDashboardModel) View() string {
	s := orchTitleStyle.Render("RECAC Orchestrator") + "\n"

	status := "Idle"
	if m.polling {
		status = "Polling..."
	}
	s += fmt.Sprintf("Status: %s | Polls: %d | Last Poll: %s\n\n",
		status, m.pollCount, m.lastPollTime.Format("15:04:05"))

	s += "Active Agents:\n"
	s += m.table.View() + "\n\n"

	s += "Events:\n"
	s += m.viewport.View() + "\n"

	s += "\n" + orchHelpStyle.Render("q: quit")
	return s
}
