package tui

import (
	"fmt"
	"recac/internal/runner"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type localTickMsg time.Time
type localSessionListMsg []*runner.SessionState
type localLogsMsg string

type localItem struct {
	session *runner.SessionState
}

func (i localItem) Title() string       { return i.session.Name }
func (i localItem) Description() string { return fmt.Sprintf("%s | PID: %d", i.session.Status, i.session.PID) }
func (i localItem) FilterValue() string { return i.session.Name }

type LocalDashboardModel struct {
	list     list.Model
	viewport viewport.Model
	sm       runner.ISessionManager
	sessions []*runner.SessionState
	selected *runner.SessionState
	logs     string
	width    int
	height   int
	focus    int // 0: List, 1: Logs
	err      error
}

func NewLocalDashboardModel(sm runner.ISessionManager) LocalDashboardModel {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Active Sessions"
	l.SetShowHelp(false)

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().PaddingLeft(1).PaddingRight(1)

	return LocalDashboardModel{
		list:     l,
		viewport: vp,
		sm:       sm,
		focus:    0,
	}
}

func (m LocalDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.fetchSessionsCmd(),
		m.tickCmd(),
	)
}

func (m LocalDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout: 30% list, 70% logs
		listWidth := int(float64(m.width) * 0.3)
		if listWidth < 20 {
			listWidth = 20
		}

		logWidth := m.width - listWidth - 4 // borders/padding

		m.list.SetSize(listWidth, m.height-2)
		m.viewport.Width = logWidth
		m.viewport.Height = m.height - 6 // title + status + borders

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			m.focus = (m.focus + 1) % 2
			return m, nil
		case "enter":
			if m.focus == 0 {
				if i, ok := m.list.SelectedItem().(localItem); ok {
					m.selected = i.session
					return m, m.fetchLogsCmd(i.session.Name)
				}
			}
		case "r":
			if m.selected != nil {
				return m, m.fetchLogsCmd(m.selected.Name)
			}
		}

	case localTickMsg:
		return m, tea.Batch(
			m.fetchSessionsCmd(),
			m.tickCmd(),
		)

	case localSessionListMsg:
		var selectedID string
		if i, ok := m.list.SelectedItem().(localItem); ok {
			selectedID = i.session.Name
		}

		m.sessions = msg
		items := make([]list.Item, len(m.sessions))
		sort.Slice(m.sessions, func(i, j int) bool {
			return m.sessions[i].StartTime.After(m.sessions[j].StartTime)
		})

		for i, s := range m.sessions {
			items[i] = localItem{session: s}
		}

		m.list.SetItems(items)

		if selectedID != "" {
			for i, it := range items {
				if it.(localItem).session.Name == selectedID {
					m.list.Select(i)
					break
				}
			}
		}

		if m.selected != nil {
			found := false
			for _, s := range m.sessions {
				if s.Name == m.selected.Name {
					m.selected = s
					found = true
					break
				}
			}
			if found && m.selected.Status == "running" {
				cmds = append(cmds, m.fetchLogsCmd(m.selected.Name))
			}
		}

	case localLogsMsg:
		newLogs := string(msg)
		if newLogs != m.logs {
			m.logs = newLogs
			m.viewport.SetContent(m.logs)
		}
	}

	var cmd tea.Cmd
	if m.focus == 0 {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m LocalDashboardModel) View() string {
	if m.width == 0 {
		return "Initializing..."
	}

	activeBorder := lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	inactiveBorder := lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "╰",
		BottomRight: "╯",
	}

	listBorderColor := subtle
	if m.focus == 0 {
		listBorderColor = highlight
	}

	logBorderColor := subtle
	if m.focus == 1 {
		logBorderColor = highlight
	}

	listViewStyle := lipgloss.NewStyle().
		Width(m.list.Width()).
		Height(m.height - 2).
		Border(activeBorder).
		BorderForeground(listBorderColor).
		MarginRight(1)

	var detailContent string
	if m.selected != nil {
		header := fmt.Sprintf(" %s ", m.selected.Name)
		status := fmt.Sprintf(" %s ", m.selected.Status)

		headerStyle := lipgloss.NewStyle().
			Bold(true).
			Background(highlight).
			Foreground(lipgloss.Color("#FFFFFF"))

		statusStyle := lipgloss.NewStyle().
			Background(statusColor(m.selected.Status)).
			Foreground(lipgloss.Color("#000000"))

		headerBar := lipgloss.JoinHorizontal(lipgloss.Top,
			headerStyle.Render(header),
			" ",
			statusStyle.Render(status),
		)

		detailContent = lipgloss.JoinVertical(lipgloss.Left,
			headerBar,
			"\n",
			m.viewport.View(),
		)
	} else {
		detailContent = lipgloss.NewStyle().
			Align(lipgloss.Center).
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Render("Select a session to view logs")
	}

	detailViewStyle := lipgloss.NewStyle().
		Width(m.viewport.Width + 2).
		Height(m.height - 2).
		Border(inactiveBorder).
		BorderForeground(logBorderColor)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		listViewStyle.Render(m.list.View()),
		detailViewStyle.Render(detailContent),
	)
}

func (m LocalDashboardModel) tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return localTickMsg(t)
	})
}

func (m LocalDashboardModel) fetchSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.sm.ListSessions()
		if err != nil {
			return nil
		}
		return localSessionListMsg(sessions)
	}
}

func (m LocalDashboardModel) fetchLogsCmd(name string) tea.Cmd {
	return func() tea.Msg {
		logs, err := m.sm.GetSessionLogContent(name, 1000)
		if err != nil {
			return localLogsMsg(fmt.Sprintf("Error fetching logs: %v", err))
		}
		return localLogsMsg(logs)
	}
}
