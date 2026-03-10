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

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
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
	viewSubmit
	viewAnalytics
	viewTree
)

type DashboardModel struct {
	host          string
	table         table.Model
	viewport      viewport.Model
	status        orchestrator.Status
	jobs          []orchestrator.JobInfo
	details       orchestrator.JobInfo
	analytics     orchestrator.Analytics
	logs          string
	logStream     io.ReadCloser
	err           error
	quitting      bool
	viewState     viewState
	showHistory   bool
	pendingJobId  string
	pendingAction string
	selectedJobs  map[string]bool

	// Submission form fields
	inputs       []textinput.Model
	textarea     textarea.Model
	focusedInput int

	// Filter fields
	filterInput textinput.Model
	isFiltering bool
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

	case analyticsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.analytics = msg.Analytics
			m.viewState = viewAnalytics
			m.viewport.SetContent(renderAnalytics(m.analytics))
			m.viewport.GotoTop()
		}
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
		if m.isFiltering {
			switch msg := msg.(type) {
			case tea.KeyMsg:
				switch msg.String() {
				case "esc":
					m.isFiltering = false
					m.filterInput.SetValue("")
					m.filterInput.Blur()
					m.updateTableContent()
					return m, tea.Batch(cmds...)
				case "enter":
					m.isFiltering = false
					m.filterInput.Blur()
					return m, tea.Batch(cmds...)
				}
				var filterCmd tea.Cmd
				m.filterInput, filterCmd = m.filterInput.Update(msg)
				cmds = append(cmds, filterCmd)
				m.updateTableContent() // Re-filter on every keystroke
				return m, tea.Batch(cmds...)
			}
			// Let other messages (tick, status, etc) fall through
		}

		// Always update filter input so blink commands work
		if m.isFiltering {
			var filterCmd tea.Cmd
			m.filterInput, filterCmd = m.filterInput.Update(msg)
			cmds = append(cmds, filterCmd)
		}

		m, cmd = m.updateMain(msg)
		cmds = append(cmds, cmd)
	case viewDetails, viewLogs, viewAnalytics, viewTree:
		m, cmd = m.updateViewport(msg)
		cmds = append(cmds, cmd)
	case viewConfirmation:
		m, cmd = m.updateConfirmation(msg)
		cmds = append(cmds, cmd)
	case viewSubmit:
		m, cmd = m.updateSubmit(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *DashboardModel) updateTableContent() {
	rows := []table.Row{}

	filterText := strings.ToLower(m.filterInput.Value())

	// Sort jobs by start time (newest first)
	sort.Slice(m.jobs, func(i, j int) bool {
		return m.jobs[i].StartTime.After(m.jobs[j].StartTime)
	})

	for _, job := range m.jobs {
		if filterText != "" {
			idMatch := strings.Contains(strings.ToLower(job.ID), filterText)
			summaryMatch := strings.Contains(strings.ToLower(job.Summary), filterText)
			if !idMatch && !summaryMatch {
				continue
			}
		}

		duration := time.Since(job.StartTime).Round(time.Second).String()
		if !job.EndTime.IsZero() {
			duration = job.EndTime.Sub(job.StartTime).Round(time.Second).String()
		}

		idDisplay := job.ID
		if m.selectedJobs[job.ID] {
			idDisplay = "[x] " + idDisplay
		} else {
			idDisplay = "[ ] " + idDisplay
		}

		rows = append(rows, table.Row{
			idDisplay,
			limitString(job.Summary, 40),
			job.Status,
			duration,
		})
	}
	m.table.SetRows(rows)
}

func (m DashboardModel) updateMain(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd

	// Helper to extract true ID by trimming prefix
	getRawID := func(displayID string) string {
		if strings.HasPrefix(displayID, "[x] ") || strings.HasPrefix(displayID, "[ ] ") {
			return displayID[4:]
		}
		return displayID
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case " ":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				m.selectedJobs[id] = !m.selectedJobs[id]
				if !m.selectedJobs[id] {
					delete(m.selectedJobs, id)
				}
				m.updateTableContent()
			}
			return m, nil
		case "v":
			// If all currently filtered/visible jobs are selected, deselect them. Otherwise, select all.
			rows := m.table.Rows()
			allSelected := true
			for _, row := range rows {
				id := getRawID(row[0])
				if !m.selectedJobs[id] {
					allSelected = false
					break
				}
			}
			for _, row := range rows {
				id := getRawID(row[0])
				if allSelected {
					delete(m.selectedJobs, id)
				} else {
					m.selectedJobs[id] = true
				}
			}
			m.updateTableContent()
			return m, nil
		case "q":
			m.quitting = true
			if m.logStream != nil {
				m.logStream.Close()
			}
			return m, tea.Quit
		case "enter":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				return m, fetchJobDetails(m.host, id)
			}
		case "l":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				return m, streamJobLogs(m.host, id)
			}
		case "o":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						if job.WorkItem.RepoURL != "" {
							return m, openBrowserCmd(job.WorkItem.RepoURL)
						}
						return m, func() tea.Msg { return actionMsg{Err: fmt.Errorf("no repo url for job %s", id)} }
					}
				}
			}
		case "/":
			m.isFiltering = true
			m.filterInput.Focus()
			return m, textinput.Blink
		case "A":
			return m, fetchAnalytics(m.host)
		case "t":
			m.viewState = viewTree
			m.viewport.SetContent(renderTree(m.jobs))
			m.viewport.GotoTop()
			return m, nil
		case "h":
			m.showHistory = !m.showHistory
			return m, fetchStatus(m.host, m.showHistory)
		case "p":
			return m, togglePause(m.host, m.status.Paused)
		case "d":
			return m, toggleDrain(m.host, m.status.Draining)
		case "f":
			return m, forcePoll(m.host)
		case "a":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_a"
				m.pendingAction = "approve multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				return m, approveJobCmd(m.host, id)
			}
		case "c":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_c"
				m.pendingAction = "cancel multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
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
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_r"
				m.pendingAction = "retry multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "retry"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "R":
			m.pendingJobId = "FAILED"
			m.pendingAction = "retry failed"
			m.viewState = viewConfirmation
			return m, nil
		case "x":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_x"
				m.pendingAction = "purge multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "purge"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "X":
			m.pendingJobId = "HISTORY"
			m.pendingAction = "clear history"
			m.viewState = viewConfirmation
			return m, nil
		case "P":
			m.pendingJobId = "PENDING"
			m.pendingAction = "clear pending"
			m.viewState = viewConfirmation
			return m, nil
		case "e":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.viewState = viewSubmit
						m.focusedInput = 0
						m.inputs[0].SetValue(job.Summary)
						m.inputs[1].SetValue(job.WorkItem.RepoURL)
						m.inputs[2].SetValue(strings.Join(job.WorkItem.DependsOn, ","))
						m.textarea.SetValue(job.WorkItem.Description)
						for i := 1; i < len(m.inputs); i++ {
							m.inputs[i].Blur()
						}
						m.textarea.Blur()
						return m, m.inputs[0].Focus()
					}
				}
			}
		case "s":
			m.viewState = viewSubmit
			m.focusedInput = 0
			// Reset form
			for i := range m.inputs {
				m.inputs[i].SetValue("")
				m.inputs[i].Blur()
			}
			m.textarea.SetValue("")
			m.textarea.Blur()

			// Focus first input
			m.inputs[0].Focus()
			return m, textinput.Blink
		case "+", "]":
			newMax := m.status.MaxConcurrentJobs + 1
			return m, scaleConcurrencyCmd(m.host, newMax)
		case "-", "[":
			if m.status.MaxConcurrentJobs > 0 {
				newMax := m.status.MaxConcurrentJobs - 1
				return m, scaleConcurrencyCmd(m.host, newMax)
			}
			return m, nil
		case ">":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_up"
				m.pendingAction = "priority multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						return m, updatePriorityCmd(m.host, id, job.WorkItem.Priority+1)
					}
				}
			}
		case "<":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_down"
				m.pendingAction = "priority multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						return m, updatePriorityCmd(m.host, id, job.WorkItem.Priority-1)
					}
				}
			}
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
			if len(m.selectedJobs) > 0 && strings.HasSuffix(m.pendingAction, "multiple") {
				var cmds []tea.Cmd
				for id := range m.selectedJobs {
					switch m.pendingAction {
					case "cancel multiple":
						cmds = append(cmds, cancelJob(m.host, id))
					case "purge multiple":
						cmds = append(cmds, purgeJobCmd(m.host, id))
					case "retry multiple":
						cmds = append(cmds, retryJob(m.host, id))
					case "approve multiple":
						cmds = append(cmds, approveJobCmd(m.host, id))
					case "priority multiple":
						// We need to fetch current priority to change it.
						for _, job := range m.jobs {
							if job.ID == id {
								if m.pendingJobId == "MULTIPLE_up" {
									cmds = append(cmds, updatePriorityCmd(m.host, id, job.WorkItem.Priority+1))
								} else if m.pendingJobId == "MULTIPLE_down" {
									cmds = append(cmds, updatePriorityCmd(m.host, id, job.WorkItem.Priority-1))
								}
								break
							}
						}
					}
				}
				m.selectedJobs = make(map[string]bool)
				m.updateTableContent()
				m.pendingJobId = ""
				m.pendingAction = ""
				m.viewState = viewMain
				return m, tea.Batch(cmds...)
			}
			var cmd tea.Cmd
			if m.pendingAction == "cancel" {
				cmd = cancelJob(m.host, m.pendingJobId)
			} else if m.pendingAction == "purge" {
				cmd = purgeJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "cancel all" {
				cmd = cancelAllJobs(m.host)
			} else if m.pendingAction == "retry" {
				cmd = retryJob(m.host, m.pendingJobId)
			} else if m.pendingAction == "retry failed" {
				cmd = retryFailedJobs(m.host)
			} else if m.pendingAction == "clear history" {
				cmd = clearHistory(m.host)
			} else if m.pendingAction == "clear pending" {
				cmd = clearPending(m.host)
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

func (m DashboardModel) updateSubmit(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc:
			m.viewState = viewMain
			return m, nil
		case tea.KeyCtrlS:
			// Submit form
			summary := m.inputs[0].Value()
			repoUrl := m.inputs[1].Value()
			dependsOnStr := m.inputs[2].Value()
			description := m.textarea.Value()

			var dependsOn []string
			if dependsOnStr != "" {
				parts := strings.Split(dependsOnStr, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						dependsOn = append(dependsOn, trimmed)
					}
				}
			}

			if summary != "" && repoUrl != "" {
				m.viewState = viewMain
				return m, submitJobCmd(m.host, summary, repoUrl, description, dependsOn)
			}
		case tea.KeyTab, tea.KeyShiftTab, tea.KeyEnter, tea.KeyUp, tea.KeyDown:
			s := msg.String()

			// Ignore Up, Down, and Enter for focus navigation if we are in the textarea
			if m.focusedInput == len(m.inputs) && (s == "up" || s == "down" || s == "enter") {
				// Let the textarea handle these keys
				break
			}

			if s == "enter" && m.focusedInput < len(m.inputs) {
				m.focusedInput++
			} else if s == "up" || s == "shift+tab" {
				m.focusedInput--
			} else if s == "down" || s == "tab" {
				m.focusedInput++
			}

			if m.focusedInput < 0 {
				m.focusedInput = len(m.inputs) // focus textarea
			} else if m.focusedInput > len(m.inputs) {
				m.focusedInput = 0 // loop back
			}

			cmds = append(cmds, m.updateFocus()...)
			return m, tea.Batch(cmds...)
		}
	}

	// Route msg to the focused component
	var cmd tea.Cmd
	if m.focusedInput < len(m.inputs) {
		m.inputs[m.focusedInput], cmd = m.inputs[m.focusedInput].Update(msg)
	} else {
		m.textarea, cmd = m.textarea.Update(msg)
	}
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m *DashboardModel) updateFocus() []tea.Cmd {
	var cmds []tea.Cmd
	for i := 0; i < len(m.inputs); i++ {
		if i == m.focusedInput {
			cmds = append(cmds, m.inputs[i].Focus())
			m.inputs[i].PromptStyle = titleStyle
			m.inputs[i].TextStyle = titleStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = lipgloss.NewStyle()
			m.inputs[i].TextStyle = lipgloss.NewStyle()
		}
	}
	if m.focusedInput == len(m.inputs) {
		cmds = append(cmds, m.textarea.Focus())
	} else {
		m.textarea.Blur()
	}
	return cmds
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
	if m.status.Draining {
		title += " [DRAINING]"
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
			msg := "No active jobs found.\n\nPress 's' to submit a new job, or 'h' to view history."
			if m.showHistory {
				msg = "No job history found.\n\nPress 's' to submit a new job, or 'h' to view active jobs."
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

		if m.isFiltering || m.filterInput.Value() != "" {
			filterView := lipgloss.NewStyle().
				MarginBottom(1).
				Render(m.filterInput.View())
			contentView = lipgloss.JoinVertical(lipgloss.Left, filterView, contentView)
		}

		helpView = statusStyle.Render("/: filter | p: pause/resume | d: drain/undrain | f: force poll | P: clear pending | +/-: scale limit | >/<: priority | h: history | A: analytics | t: tree | enter: details | l: logs | o: open repo | a: approve | c: cancel | C: cancel all | r: retry | R: retry failed | x: purge | X: clear history | e: edit/clone | s: submit | q: quit")
	case viewDetails:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalytics:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewTree:
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
		} else if m.pendingAction == "purge" {
			dialogMsg = fmt.Sprintf("Are you sure you want to PURGE job %s entirely?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingJobId)
		} else if m.pendingAction == "clear pending" {
			dialogMsg = "Are you sure you want to clear ALL pending jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)"
		} else if m.pendingAction == "cancel multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to CANCEL %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "purge multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to PURGE %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "retry multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to RETRY %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "approve multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to APPROVE %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "priority multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to CHANGE PRIORITY for %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
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
			Height(m.viewport.Height+5). // approximate table height
			Align(lipgloss.Center, lipgloss.Center)

		contentView = containerStyle.Render(contentView)

		helpView = statusStyle.Render("y/enter: confirm | n/q/esc: cancel")
	case viewSubmit:
		var sb strings.Builder
		sb.WriteString("Submit Ad-hoc Job\n\n")

		for i := range m.inputs {
			sb.WriteString(m.inputs[i].View())
			sb.WriteString("\n\n")
		}

		sb.WriteString("Description:\n")
		sb.WriteString(m.textarea.View())

		contentView = baseStyle.Render(sb.String())
		helpView = statusStyle.Render("tab/shift+tab/up/down: focus | ctrl+s: submit | esc: cancel")
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

func toggleDrain(host string, isDraining bool) tea.Cmd {
	return func() tea.Msg {
		endpoint := "/drain"
		if isDraining {
			endpoint = "/undrain"
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
		action := "Draining"
		if isDraining {
			action = "Undrained"
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

func scaleConcurrencyCmd(host string, max int) tea.Cmd {
	return func() tea.Msg {
		reqBody := fmt.Sprintf(`{"max_concurrent_jobs": %d}`, max)
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/scale", host), strings.NewReader(reqBody))
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
		return actionMsg{Message: fmt.Sprintf("Scaled concurrency to %d", max)}
	}
}

func approveJobCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/approve", host, id), nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}
		return actionMsg{Message: "Approved"}
	}
}

func updatePriorityCmd(host, id string, newPriority int) tea.Cmd {
	return func() tea.Msg {
		urlStr := fmt.Sprintf("%s/jobs/%s/priority", host, id)
		reqBody := fmt.Sprintf(`{"priority": %d}`, newPriority)

		req, err := http.NewRequest(http.MethodPut, urlStr, strings.NewReader(reqBody))
		if err != nil {
			return actionMsg{Err: err}
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Updated priority for job %s to %d", id, newPriority)}
	}
}

func purgeJobCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/history/%s", host, id), nil)
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
		return actionMsg{Message: "Purged"}
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

func clearPending(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/pending", host), nil)
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

		return actionMsg{Message: fmt.Sprintf("Cleared %d pending jobs", int(cleared))}
	}
}

// Allow mocking in tests
var utilsOpenBrowser = utils.OpenBrowser

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		err := utilsOpenBrowser(url)
		if err != nil {
			return actionMsg{Err: err}
		}
		return actionMsg{Message: "Opened browser"}
	}
}

func submitJobCmd(host, summary, repoUrl, description string, dependsOn []string) tea.Cmd {
	return func() tea.Msg {
		// Use a timestamp-based ID or a unique ID.
		// For simplicity, generating an ad-hoc ID
		jobID := fmt.Sprintf("adhoc-%d", time.Now().Unix())

		item := orchestrator.WorkItem{
			ID:          jobID,
			Summary:     summary,
			RepoURL:     repoUrl,
			Description: description,
			DependsOn:   dependsOn,
		}

		bodyBytes, err := json.Marshal(item)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs", host), strings.NewReader(string(bodyBytes)))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			respBody, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))}
		}

		return actionMsg{Message: "Job submitted successfully"}
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
	// Initialize inputs for submission form
	inputs := make([]textinput.Model, 3)

	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Fix login issue"
	inputs[0].Prompt = "Summary: "
	inputs[0].Focus() // Default focus
	inputs[0].Width = 50

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "https://github.com/org/repo"
	inputs[1].Prompt = "Repo URL: "
	inputs[1].Width = 50

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "JOB-1,JOB-2"
	inputs[2].Prompt = "Depends On (comma-separated IDs): "
	inputs[2].Width = 50

	ta := textarea.New()
	ta.Placeholder = "Detailed description of the issue..."
	ta.SetHeight(10)
	ta.SetWidth(60)

	columns := []table.Column{
		{Title: "ID", Width: 19}, // Increased width for [x] indicator
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

	fi := textinput.New()
	fi.Placeholder = "Filter jobs by ID or Summary..."
	fi.Prompt = "/"
	fi.Width = 40

	return DashboardModel{
		host:         host,
		table:        t,
		viewport:     vp,
		viewState:    viewMain,
		inputs:       inputs,
		textarea:     ta,
		filterInput:  fi,
		isFiltering:  false,
		selectedJobs: make(map[string]bool),
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

type analyticsMsg struct {
	Analytics orchestrator.Analytics
	Err       error
}

func fetchAnalytics(host string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("%s/analytics", host))
		if err != nil {
			return analyticsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return analyticsMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var analytics orchestrator.Analytics
		if err := json.NewDecoder(resp.Body).Decode(&analytics); err != nil {
			return analyticsMsg{Err: err}
		}

		return analyticsMsg{Analytics: analytics}
	}
}

func renderAnalytics(a orchestrator.Analytics) string {
	s := strings.Builder{}
	h1 := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render
	kv := func(k, v string) string {
		return fmt.Sprintf("%s: %s\n", h1(k), v)
	}

	s.WriteString(h1("Orchestrator Analytics") + "\n\n")
	s.WriteString(kv("Total Jobs", fmt.Sprintf("%d", a.TotalJobs)))
	s.WriteString(kv("Successful Jobs", fmt.Sprintf("%d", a.SuccessfulJobs)))
	s.WriteString(kv("Failed Jobs", fmt.Sprintf("%d", a.FailedJobs)))
	s.WriteString(kv("Canceled Jobs", fmt.Sprintf("%d", a.CanceledJobs)))
	s.WriteString(kv("Success Rate", fmt.Sprintf("%.2f%%", a.SuccessRate)))
	s.WriteString(kv("Average Duration", a.AverageDuration.Round(time.Second).String()))

	return s.String()
}

func renderTree(jobs []orchestrator.JobInfo) string {
	if len(jobs) == 0 {
		return "No jobs found."
	}

	jobMap := make(map[string]orchestrator.JobInfo)
	childrenMap := make(map[string][]string)

	for _, job := range jobs {
		jobMap[job.ID] = job
		for _, dep := range job.WorkItem.DependsOn {
			childrenMap[dep] = append(childrenMap[dep], job.ID)
		}
	}

	var rootJobs []string
	for _, job := range jobs {
		if len(job.WorkItem.DependsOn) == 0 {
			rootJobs = append(rootJobs, job.ID)
		} else {
			allDepsMissing := true
			for _, dep := range job.WorkItem.DependsOn {
				if _, exists := jobMap[dep]; exists {
					allDepsMissing = false
					break
				}
			}
			if allDepsMissing {
				rootJobs = append(rootJobs, job.ID)
			}
		}
	}

	var sb strings.Builder
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Job Dependency Tree (%d Jobs)", len(jobs))))
	sb.WriteString("\n\n")

	for _, root := range rootJobs {
		renderNode(root, jobMap, childrenMap, "", true, &sb)
	}

	return sb.String()
}

func renderNode(nodeID string, jobMap map[string]orchestrator.JobInfo, childrenMap map[string][]string, prefix string, isLast bool, sb *strings.Builder) {
	job, exists := jobMap[nodeID]
	if !exists {
		return
	}

	branch := "├── "
	if isLast {
		branch = "└── "
	}

	idStyle := lipgloss.NewStyle().Bold(true)

	statusColor := "252"
	switch job.Status {
	case "Completed":
		statusColor = "42" // Green
	case "Failed":
		statusColor = "196" // Red
	case "Pending":
		statusColor = "214" // Orange
	case "Spawning", "Running", "Active":
		statusColor = "39" // Blue
	}

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor))
	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	nodeStr := fmt.Sprintf("%s (%s) %s",
		idStyle.Render(job.ID),
		statusStyle.Render(job.Status),
		summaryStyle.Render(limitString(job.Summary, 40)),
	)

	sb.WriteString(fmt.Sprintf("%s%s%s\n", prefix, branch, nodeStr))

	childPrefix := prefix
	if isLast {
		childPrefix += "    "
	} else {
		childPrefix += "│   "
	}

	children := childrenMap[nodeID]
	for i, child := range children {
		renderNode(child, jobMap, childrenMap, childPrefix, i == len(children)-1, sb)
	}
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
