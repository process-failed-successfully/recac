package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- DI for session loading ---
var (
	sessionLoader SessionLoader
)

// SetSessionLoader allows the main package to inject the session loading function.
func SetSessionLoader(loader SessionLoader) {
	sessionLoader = loader
}

// --- Bubbletea Model ---

// Styles are now defined in styles.go

// sessionItem implements the list.Item interface.
type sessionItem Session

func (i sessionItem) FilterValue() string { return i.Name }

// itemDelegate handles rendering of items in the list.
type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(sessionItem)
	if !ok {
		return
	}

	str := fmt.Sprintf("%-25s %-10s %-8s %-10s %s", i.Name, i.Status, i.Location, i.StartTime, i.Cost)

	if index == m.Index() {
		fmt.Fprint(w, selectedItemStyle.Render("> "+str))
	} else {
		fmt.Fprint(w, itemStyle.Render(str))
	}
}

// sessionsLoadedMsg is sent when the async session loading is complete.
type sessionsLoadedMsg struct {
	sessions []list.Item
	err      error
}

// DashboardModel is the main model for the TUI dashboard.
type DashboardModel struct {
	list     list.Model
	spinner  spinner.Model
	loading  bool
	err      error
	quitting bool
}

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel() DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return DashboardModel{
		spinner: s,
		loading: true,
	}
}

// loadSessionsCmd is a tea.Cmd that calls the injected session loader.
func loadSessionsCmd() tea.Msg {
	if sessionLoader == nil {
		return sessionsLoadedMsg{err: fmt.Errorf("session loader not initialized")}
	}
	sessions, err := sessionLoader()
	if err != nil {
		return sessionsLoadedMsg{err: err}
	}

	items := make([]list.Item, len(sessions))
	for i, s := range sessions {
		items[i] = sessionItem(s)
	}
	return sessionsLoadedMsg{sessions: items}
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(loadSessionsCmd, m.spinner.Tick)
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case sessionsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		const defaultWidth = 20
		const defaultHeight = 14

		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()

		m.list = list.New(msg.sessions, itemDelegate{}, defaultWidth-h, defaultHeight-v)
		m.list.Title = "RECAC Sessions"
		m.list.SetShowStatusBar(false)
		m.list.SetFilteringEnabled(true)
		m.list.Styles.Title = titleStyle
		m.list.Styles.PaginationStyle = paginationStyle
		m.list.Styles.HelpStyle = helpStyle
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		if m.loading {
			m.spinner, cmd = m.spinner.Update(msg)
		}
		return m, cmd
	}

	var cmd tea.Cmd
	if !m.loading && m.err == nil {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m DashboardModel) View() string {
	if m.quitting {
		return quitTextStyle.Render("Closing dashboard...")
	}
	if m.loading {
		return fmt.Sprintf("\n   %s Loading sessions...\n\n", m.spinner.View())
	}
	if m.err != nil {
		return fmt.Sprintf("\n   Error loading sessions: %v\n\n", m.err)
	}

	var leftPane, rightPane string

	listContainerStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2)
	leftPane = listContainerStyle.Render(m.list.View())

	detailView := "Select a session to see details."
	if i, ok := m.list.SelectedItem().(sessionItem); ok {
		detailView = detailTitleStyle.Render(i.Name) + "\n" + detailTextStyle.Render(i.Details)
	}
	detailContainerStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(1, 2)
	rightPane = detailContainerStyle.Render(strings.TrimSpace(detailView))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}
