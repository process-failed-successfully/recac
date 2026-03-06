package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
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

	detailsStyle = lipgloss.NewStyle().
		Padding(1, 2)
)

type viewState int

const (
	viewMain viewState = iota
	viewDetails
	viewLogs
	viewConfirmation
)

type DashboardModel struct {
	host          string
	table         table.Model
	viewport      viewport.Model
	status        orchestrator.Status
	jobs          []orchestrator.JobInfo
	details       orchestrator.JobInfo
	logs          string
	logStream     io.ReadCloser
	err           error
	quitting      bool
	viewState     viewState
	showHistory   bool
	pendingJobId  string
	pendingAction string
}

type tickMsg time.Time

type statusMsg struct {
	Status orchestrator.Status
	Jobs   []orchestrator.JobInfo
	Err    error
}

type detailsMsg struct {
	Job orchestrator.JobInfo
	Err error
}

type logStreamMsg struct {
	Stream io.ReadCloser
	Err    error
}

type logChunkMsg struct {
	Chunk string
	Err   error
}

type logFinishedMsg struct {
	Err error
}

type actionMsg struct {
	Message string
	Err     error
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tick(),
		fetchStatus(m.host, m.showHistory),
	)
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			if m.logStream != nil {
				m.logStream.Close()
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.table.SetWidth(msg.Width)
		m.table.SetHeight(msg.Height - 5) // Subtract header/footer
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 5

	case tickMsg:
		cmds = append(cmds, tick())
		cmds = append(cmds, fetchStatus(m.host, m.showHistory))
		return m, tea.Batch(cmds...)

	case statusMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.status = msg.Status
			m.jobs = msg.Jobs
			m.err = nil
			m.updateTableContent()
		}
		// Continue to update view specific models if needed, but table update is handled via m.updateTableContent
		return m, nil

	case detailsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.details = msg.Job
			m.viewState = viewDetails
			m.viewport.SetContent(renderDetails(m.details))
			m.viewport.GotoTop()
		}
		return m, nil

	case logStreamMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			if m.logStream != nil {
				m.logStream.Close()
			}
			m.logStream = msg.Stream
			m.logs = "" // Clear previous logs
			m.viewport.SetContent("")
			m.viewState = viewLogs
			// Start reading chunks
			cmds = append(cmds, waitForLogChunk(m.logStream))
		}
		return m, tea.Batch(cmds...)

	case logChunkMsg:
		if msg.Chunk != "" {
			m.logs += msg.Chunk
			m.viewport.SetContent(m.logs)
			m.viewport.GotoBottom()
		}

		if msg.Err != nil {
			// If stream ended or error occurred during read
			if msg.Err != io.EOF {
				m.err = msg.Err
			}
			if m.logStream != nil {
				m.logStream.Close()
				m.logStream = nil
			}
		} else {
			// Continue reading
			if m.logStream != nil {
				cmds = append(cmds, waitForLogChunk(m.logStream))
			}
		}
		return m, tea.Batch(cmds...)

	case logFinishedMsg:
		if m.logStream != nil {
			m.logStream.Close()
			m.logStream = nil
		}
		if msg.Err != nil && msg.Err != io.EOF {
			m.err = msg.Err
		}
		return m, nil

	case actionMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			// Maybe show a status message? For now just refresh
			cmds = append(cmds, fetchStatus(m.host, m.showHistory))
		}
		return m, tea.Batch(cmds...)
	}

	// View-specific logic
	var cmd tea.Cmd
	switch m.viewState {
	case viewMain:
		m, cmd = m.updateMain(msg)
		cmds = append(cmds, cmd)
	case viewDetails, viewLogs:
		m, cmd = m.updateViewport(msg)
		cmds = append(cmds, cmd)
	case viewConfirmation:
		m, cmd = m.updateConfirmation(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *DashboardModel) updateTableContent() {
	rows := []table.Row{}
	// Sort jobs by start time (newest first)
	sort.Slice(m.jobs, func(i, j int) bool {
		return m.jobs[i].StartTime.After(m.jobs[j].StartTime)
	})

	for _, job := range m.jobs {
		duration := time.Since(job.StartTime).Round(time.Second).String()
		if !job.EndTime.IsZero() {
			duration = job.EndTime.Sub(job.StartTime).Round(time.Second).String()
		}
		rows = append(rows, table.Row{
			job.ID,
			limitString(job.Summary, 40),
			job.Status,
			duration,
		})
	}
	m.table.SetRows(rows)
}

func (m DashboardModel) updateMain(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			m.quitting = true
			if m.logStream != nil {
				m.logStream.Close()
			}
			return m, tea.Quit
		case "enter":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := selected[0]
				return m, fetchJobDetails(m.host, id)
			}
		case "l":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := selected[0]
				return m, streamJobLogs(m.host, id)
			}
		case "h":
			m.showHistory = !m.showHistory
			return m, fetchStatus(m.host, m.showHistory)
		case "p":
			return m, togglePause(m.host, m.status.Paused)
		case "f":
			return m, forcePoll(m.host)
		case "c":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = selected[0]
				m.pendingAction = "cancel"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "C":
			m.pendingJobId = "ALL"
			m.pendingAction = "cancel all"
			m.viewState = viewConfirmation
			return m, nil
		case "r":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = selected[0]
				m.pendingAction = "retry"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "R":
			m.pendingJobId = "FAILED"
			m.pendingAction = "retry failed"
			m.viewState = viewConfirmation
			return m, nil
		case "X":
			m.pendingJobId = "HISTORY"
			m.pendingAction = "clear history"
			m.viewState = viewConfirmation
			return m, nil
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateConfirmation(msg tea.Msg) (DashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "enter":
			var cmd tea.Cmd
			if m.pendingAction == "cancel" {
				cmd = cancelJob(m.host, m.pendingJobId)
			} else if m.pendingAction == "cancel all" {
				cmd = cancelAllJobs(m.host)
			} else if m.pendingAction == "retry" {
				cmd = retryJob(m.host, m.pendingJobId)
			} else if m.pendingAction == "retry failed" {
				cmd = retryFailedJobs(m.host)
			} else if m.pendingAction == "clear history" {
				cmd = clearHistory(m.host)
			}
			m.pendingJobId = ""
			m.pendingAction = ""
			m.viewState = viewMain
			return m, cmd
		case "n", "q", "esc":
			m.pendingJobId = ""
			m.pendingAction = ""
			m.viewState = viewMain
			return m, nil
		}
	}
	return m, nil
}

func (m DashboardModel) updateViewport(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			if m.logStream != nil {
				m.logStream.Close()
				m.logStream = nil
			}
			m.viewState = viewMain
			return m, nil
		}
	}
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m DashboardModel) View() string {
	if m.quitting {
		return "Exiting dashboard...\n"
	}

	title := "Orchestrator Dashboard"
	if m.status.Paused {
		title += " [PAUSED]"
	}

	header := fmt.Sprintf(
		"%s\nHost: %s | Uptime: %s | Poll Interval: %s | Active Jobs: %d | Total Spawns: %d\nLast Poll: %s (%d items)",
		title,
		m.host,
		m.status.Uptime,
		m.status.PollInterval,
		m.status.ActiveSpawns,
		m.status.TotalSpawns,
		m.status.LastPoll.Format("15:04:05"),
		m.status.LastPollItems,
	)

	var contentView string
	var helpView string

	switch m.viewState {
	case viewMain:
		if len(m.jobs) == 0 {
			msg := "No active jobs found.\n\nPress 'h' to view history."
			if m.showHistory {
				msg = "No job history found.\n\nPress 'h' to view active jobs."
			}

			// Create a style that mimics the table's dimensions to prevent layout shifts
			// Use viewport width/height as fallback or primary if table dimensions aren't reliable when empty
			width := m.viewport.Width
			if width == 0 {
				width = 85 // Fallback to default column sum
			}
			height := m.viewport.Height
			if height == 0 {
				height = 10 // Fallback
			}

			emptyStyle := lipgloss.NewStyle().
				Align(lipgloss.Center).
				Width(width).
				Height(height).
				PaddingTop(height / 3).
				Foreground(lipgloss.Color("241"))

			contentView = baseStyle.Render(emptyStyle.Render(msg))
		} else {
			contentView = baseStyle.Render(m.table.View())
		}
		helpView = statusStyle.Render("p: pause/resume | f: force poll | h: history | enter: details | l: logs | c: cancel | C: cancel all | r: retry | R: retry failed | X: clear history | q: quit")
	case viewDetails:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewLogs:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back | streaming logs...")
	case viewConfirmation:
		// Keep showing the main table in the background
		contentView = baseStyle.Render(m.table.View())

		// Create a modal dialog
		var dialogMsg string
		if m.pendingAction == "cancel all" {
			dialogMsg = "Are you sure you want to cancel ALL active jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)"
		} else if m.pendingAction == "retry failed" {
			dialogMsg = "Are you sure you want to retry ALL failed jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)"
		} else if m.pendingAction == "clear history" {
			dialogMsg = "Are you sure you want to clear ALL job history?\n\n(y/Enter: confirm, n/q/Esc: cancel)"
		} else {
			dialogMsg = fmt.Sprintf("Are you sure you want to %s job %s?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingAction, m.pendingJobId)
		}

		dialogWidth := 50
		dialogHeight := 7

		dialogStyle := lipgloss.NewStyle().
			Width(dialogWidth).
			Height(dialogHeight).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		// Overlay logic would be complex here without a layer manager,
		// so for now we just replace the content view or render it differently.
		// A simple way is to just render the dialog.
		contentView = dialogStyle.Render(dialogMsg)

		// If we want to center it nicely we might need more layout logic,
		// but standard center alignment in the container usually works.
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height + 5). // approximate table height
			Align(lipgloss.Center, lipgloss.Center)

		contentView = containerStyle.Render(contentView)

		helpView = statusStyle.Render("y/enter: confirm | n/q/esc: cancel")
	}

	if m.err != nil {
		helpView = statusStyle.Foreground(lipgloss.Color("196")).Render(fmt.Sprintf("Error: %v", m.err))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(header),
		contentView,
		helpView,
	) + "\n"
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchStatus(host string, history bool) tea.Cmd {
	return func() tea.Msg {
		sResp, err := http.Get(fmt.Sprintf("%s/status", host))
		if err != nil {
			return statusMsg{Err: err}
		}
		defer sResp.Body.Close()

		var status orchestrator.Status
		if err := json.NewDecoder(sResp.Body).Decode(&status); err != nil {
			return statusMsg{Err: err}
		}

		url := fmt.Sprintf("%s/jobs", host)
		if history {
			url += "?state=all"
		}
		jResp, err := http.Get(url)
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

func fetchJobDetails(host, id string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, id))
		if err != nil {
			return detailsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return detailsMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}

		var job orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			return detailsMsg{Err: err}
		}
		return detailsMsg{Job: job}
	}
}

func streamJobLogs(host, id string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, id))
		if err != nil {
			return logStreamMsg{Err: err}
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return logStreamMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return logStreamMsg{Stream: resp.Body}
	}
}

func waitForLogChunk(r io.Reader) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096) // 4KB chunks
		n, err := r.Read(buf)
		return logChunkMsg{Chunk: string(buf[:n]), Err: err}
	}
}

func togglePause(host string, isPaused bool) tea.Cmd {
	return func() tea.Msg {
		endpoint := "/pause"
		if isPaused {
			endpoint = "/resume"
		}
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s%s", host, endpoint), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}
		action := "Paused"
		if isPaused {
			action = "Resumed"
		}
		return actionMsg{Message: action}
	}
}

func forcePoll(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/poll", host), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}
		return actionMsg{Message: "Poll triggered"}
	}
}

func cancelJob(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s", host, id), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}
		return actionMsg{Message: "Cancelled"}
	}
}

func cancelAllJobs(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs", host), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return actionMsg{Err: fmt.Errorf("failed to parse response: %v", err)}
		}
		canceled, ok := result["canceled"].(float64)
		if !ok {
			return actionMsg{Err: fmt.Errorf("invalid response format")}
		}

		return actionMsg{Message: fmt.Sprintf("Cancelled %d Jobs", int(canceled))}
	}
}

func retryJob(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/retry", host, id), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}
		return actionMsg{Message: "Retried"}
	}
}

func clearHistory(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/history", host), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return actionMsg{Err: fmt.Errorf("failed to parse response: %v", err)}
		}
		cleared, ok := result["cleared"].(float64)
		if !ok {
			return actionMsg{Err: fmt.Errorf("invalid response format")}
		}

		return actionMsg{Message: fmt.Sprintf("Cleared %d jobs", int(cleared))}
	}
}

func retryFailedJobs(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/retry-failed", host), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return actionMsg{Err: fmt.Errorf("failed to parse response: %v", err)}
		}
		retried, ok := result["retried"].(float64)
		if !ok {
			return actionMsg{Err: fmt.Errorf("invalid response format")}
		}

		return actionMsg{Message: fmt.Sprintf("Retried %d failed jobs", int(retried))}
	}
}

// NewDashboardModel initializes a new DashboardModel with default styles
func NewDashboardModel(host string) DashboardModel {
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

	vp := viewport.New(100, 20)
	vp.Style = lipgloss.NewStyle().
		Padding(1, 2)

	return DashboardModel{
		host:      host,
		table:     t,
		viewport:  vp,
		viewState: viewMain,
	}
}

func StartDashboard(host string) error {
	m := NewDashboardModel(host)
	// Enable alt screen for full screen view
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
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

func renderDetails(job orchestrator.JobInfo) string {
	s := strings.Builder{}

	h1 := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render
	kv := func(k, v string) string {
		return fmt.Sprintf("%s: %s\n", h1(k), v)
	}

	s.WriteString(kv("ID", job.ID))
	s.WriteString(kv("Summary", job.Summary))
	s.WriteString(kv("Status", job.Status))
	s.WriteString(kv("Start Time", job.StartTime.Format(time.RFC3339)))
	if !job.EndTime.IsZero() {
		s.WriteString(kv("End Time", job.EndTime.Format(time.RFC3339)))
		s.WriteString(kv("Duration", job.EndTime.Sub(job.StartTime).String()))
	} else {
		s.WriteString(kv("Duration", time.Since(job.StartTime).String()))
	}
	if job.Error != "" {
		s.WriteString(kv("Error", job.Error))
	}
	s.WriteString("\n")

	s.WriteString(h1("Work Item Details") + "\n")
	s.WriteString(kv("Repo URL", job.WorkItem.RepoURL))
	s.WriteString(kv("Description", job.WorkItem.Description))

	if len(job.WorkItem.EnvVars) > 0 {
		s.WriteString("\n" + h1("Environment Variables") + "\n")
		for k, v := range job.WorkItem.EnvVars {
			s.WriteString(fmt.Sprintf("  %s=%s\n", k, v))
		}
	}

	return s.String()
}
