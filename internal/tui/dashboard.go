package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Margin(1, 0)

	// Allow mocking http.Get
	statusFetcher = func(url string) (*http.Response, error) {
		return http.Get(url)
	}

	// Allow mocking bubbletea run
	programRunner = func(p *tea.Program) (tea.Model, error) {
		return p.Run()
	}
)

type DashboardModel struct {
	host     string
	table    table.Model
	status   orchestrator.Status
	jobs     []orchestrator.JobInfo
	err      error
	quitting bool
}

type tickMsg time.Time

type statusMsg struct {
	Status orchestrator.Status
	Jobs   []orchestrator.JobInfo
	Err    error
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tick(),
		fetchStatus(m.host),
	)
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}

	case tickMsg:
		return m, tea.Batch(
			tick(),
			fetchStatus(m.host),
		)

	case statusMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.status = msg.Status
		m.jobs = msg.Jobs
		m.err = nil

		// Update table
		rows := []table.Row{}
		// Sort jobs by start time (newest first)
		sort.Slice(m.jobs, func(i, j int) bool {
			return m.jobs[i].StartTime.After(m.jobs[j].StartTime)
		})

		for _, job := range m.jobs {
			duration := time.Since(job.StartTime).Round(time.Second).String()
			rows = append(rows, table.Row{
				job.ID,
				limitString(job.Summary, 40),
				job.Status,
				duration,
			})
		}
		m.table.SetRows(rows)
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m DashboardModel) View() string {
	if m.quitting {
		return "Exiting dashboard...\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error polling orchestrator: %v\n\nPress q to quit.", m.err)
	}

	header := fmt.Sprintf(
		"Orchestrator Dashboard\nHost: %s | Uptime: %s | Poll Interval: %s | Active Jobs: %d | Total Spawns: %d\nLast Poll: %s (%d items)",
		m.host,
		m.status.Uptime,
		m.status.PollInterval,
		m.status.ActiveSpawns,
		m.status.TotalSpawns,
		m.status.LastPoll.Format("15:04:05"),
		m.status.LastPollItems,
	)

	return baseStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(header),
			m.table.View(),
			statusStyle.Render("Press q to quit."),
		),
	) + "\n"
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchStatus(host string) tea.Cmd {
	return func() tea.Msg {
		// Fetch Status
		sResp, err := statusFetcher(fmt.Sprintf("%s/status", host))
		if err != nil {
			return statusMsg{Err: err}
		}
		defer sResp.Body.Close()

		var status orchestrator.Status
		if err := json.NewDecoder(sResp.Body).Decode(&status); err != nil {
			return statusMsg{Err: err}
		}

		// Fetch Jobs
		jResp, err := statusFetcher(fmt.Sprintf("%s/jobs", host))
		if err != nil {
			return statusMsg{Err: err}
		}
		defer jResp.Body.Close()

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(jResp.Body).Decode(&jobs); err != nil {
			return statusMsg{Err: err}
		}

		return statusMsg{Status: status, Jobs: jobs}
	}
}

func StartDashboard(host string) error {
	columns := []table.Column{
		{Title: "ID", Width: 15},
		{Title: "Summary", Width: 40},
		{Title: "Status", Width: 15},
		{Title: "Duration", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	s := table.DefaultStyles()
	s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	p := tea.NewProgram(DashboardModel{
		host:  host,
		table: t,
	})

	if _, err := programRunner(p); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
	return nil
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
