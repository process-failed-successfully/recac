package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

// Orchestrator Messages
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

// OrchestratorObserver implements orchestrator.Observer and sends messages to a channel
type OrchestratorObserver struct {
	Ch chan tea.Msg
}

func NewOrchestratorObserver() *OrchestratorObserver {
	return &OrchestratorObserver{
		Ch: make(chan tea.Msg, 100),
	}
}

func (o *OrchestratorObserver) OnPollStart() {
	select {
	case o.Ch <- PollStartMsg{}:
	default:
	}
}

func (o *OrchestratorObserver) OnPollEnd(count int, err error) {
	select {
	case o.Ch <- PollEndMsg{Count: count, Err: err}:
	default:
	}
}

func (o *OrchestratorObserver) OnSpawnStart(item orchestrator.WorkItem) {
	select {
	case o.Ch <- SpawnStartMsg{Item: item}:
	default:
	}
}

func (o *OrchestratorObserver) OnSpawnEnd(item orchestrator.WorkItem, err error) {
	select {
	case o.Ch <- SpawnEndMsg{Item: item, Err: err}:
	default:
	}
}

func waitForActivity(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

// OrchestratorDashboardModel is the main model for the TUI
type OrchestratorDashboardModel struct {
	observer     *OrchestratorObserver
	monitorModel MonitorDashboardModel
	viewport     viewport.Model

	events       []string
	status       string
	lastPoll     time.Time

	activeTab    int // 0: Events, 1: Sessions
	width, height int
}

func NewOrchestratorDashboardModel(obs *OrchestratorObserver, monitor MonitorDashboardModel) OrchestratorDashboardModel {
	vp := viewport.New(0, 0)
	return OrchestratorDashboardModel{
		observer:     obs,
		monitorModel: monitor,
		viewport:     vp,
		events:       []string{},
		status:       "Initializing...",
	}
}

func (m OrchestratorDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		waitForActivity(m.observer.Ch),
		m.monitorModel.Init(),
	)
}

func (m OrchestratorDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.activeTab = (m.activeTab + 1) % 2
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case PollStartMsg:
		m.status = "Polling..."
		return m, waitForActivity(m.observer.Ch)

	case PollEndMsg:
		m.lastPoll = time.Now()
		if msg.Err != nil {
			m.status = fmt.Sprintf("Poll Failed: %v", msg.Err)
			m.addEvent(fmt.Sprintf("❌ Poll Failed: %v", msg.Err))
		} else {
			m.status = fmt.Sprintf("Poll Success (%d items)", msg.Count)
			if msg.Count > 0 {
				m.addEvent(fmt.Sprintf("✅ Found %d items", msg.Count))
			}
		}
		return m, waitForActivity(m.observer.Ch)

	case SpawnStartMsg:
		m.addEvent(fmt.Sprintf("🚀 Spawning agent for %s", msg.Item.ID))
		return m, waitForActivity(m.observer.Ch)

	case SpawnEndMsg:
		if msg.Err != nil {
			m.addEvent(fmt.Sprintf("❌ Spawn Failed for %s: %v", msg.Item.ID, msg.Err))
		} else {
			m.addEvent(fmt.Sprintf("✨ Spawned agent for %s", msg.Item.ID))
		}
		return m, waitForActivity(m.observer.Ch)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 5 // Reserve header space

		// Update monitor model size
		monitorMsg := msg
		monitorMsg.Height = m.height - 5 // Adjust for header
		updatedMonitor, monitorCmd := m.monitorModel.Update(monitorMsg)
		m.monitorModel = updatedMonitor.(MonitorDashboardModel)
		cmds = append(cmds, monitorCmd)
		return m, tea.Batch(cmds...)
	}

	// Forward messages to monitor model if needed (e.g. tick messages)
	// Monitor handles its own ticks and session refreshes.
	// We should always forward generic messages or if it's the active tab?
	// Ticks should always be forwarded.

	updatedMonitor, monitorCmd := m.monitorModel.Update(msg)
	m.monitorModel = updatedMonitor.(MonitorDashboardModel)
	cmds = append(cmds, monitorCmd)

	return m, tea.Batch(cmds...)
}

func (m *OrchestratorDashboardModel) addEvent(evt string) {
	timestamp := time.Now().Format("15:04:05")
	m.events = append([]string{fmt.Sprintf("[%s] %s", timestamp, evt)}, m.events...)
	if len(m.events) > 100 {
		m.events = m.events[:100]
	}
	// Update viewport content
	content := ""
	for _, e := range m.events {
		content += e + "\n"
	}
	m.viewport.SetContent(content)
}

var (
	orchHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	orchTabStyle    = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("240"))
	orchActiveTabStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("205")).Bold(true).Underline(true)
)

func (m OrchestratorDashboardModel) View() string {
	// Tabs
	eventsTab := orchTabStyle.Render("Events")
	sessionsTab := orchTabStyle.Render("Sessions")
	if m.activeTab == 0 {
		eventsTab = orchActiveTabStyle.Render("Events")
	} else {
		sessionsTab = orchActiveTabStyle.Render("Sessions")
	}

	header := fmt.Sprintf("%s\nStatus: %s | Last Poll: %s\n%s | %s\n",
		orchHeaderStyle.Render("Recac Orchestrator"),
		m.status,
		m.lastPoll.Format("15:04:05"),
		eventsTab, sessionsTab,
	)

	var content string
	if m.activeTab == 0 {
		content = m.viewport.View()
	} else {
		content = m.monitorModel.View()
	}

	return header + "\n" + content
}
