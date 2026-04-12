package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"recac/internal/orchestrator"

	"recac/internal/utils"

	"github.com/charmbracelet/glamour"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	clipboardWriteAll = clipboard.WriteAll

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
	viewTimeoutInput
	viewSimulate
	viewDepsInput
	viewEnvInput
	viewTagsInput
	viewAgentInput
	viewRenameInput
	viewMaxRetriesInput
	viewExplain
	viewCompare
	viewSearchLogsInput
	viewSearchLogsContextInput
	viewSearchLogsResult
	viewSearchJobsInput
	viewSearchJobsResult
	viewAnalyzeFailures
	viewCriticalPath
	viewBlockers
	viewDependents
	viewTags
	viewAnalyzeDurations
	viewAnalyzeReliability
	viewAnalyzeCosts
	viewAnalyzeAgents
	viewDeletePendingGroupInput
	viewDeletePendingTagInput
	viewDeletePendingMatchInput
	viewPauseGroupInput
	viewResumeGroupInput
	viewSummary
)

type DurationStats struct {
	TotalJobs      int       `json:"total_jobs"`
	TotalDuration  float64   `json:"total_duration_ms"`
	MeanDuration   float64   `json:"mean_duration_ms"`
	MedianDuration float64   `json:"median_duration_ms"`
	MinDuration    float64   `json:"min_duration_ms"`
	MaxDuration    float64   `json:"max_duration_ms"`
	TagStats       []struct {
		Tag          string  `json:"tag"`
		Count        int     `json:"count"`
		MeanDuration float64 `json:"mean_duration_ms"`
	} `json:"tag_stats"`
	TopSlowest []orchestrator.JobInfo `json:"top_slowest"`
}

type FlakyJobStat struct {
	Summary      string  `json:"summary"`
	Occurrences  int     `json:"occurrences"`
	TotalRetries int     `json:"total_retries"`
	AvgRetries   float64 `json:"avg_retries"`
}

type FailedJobStat struct {
	Summary     string `json:"summary"`
	Occurrences int    `json:"occurrences"`
}

type ReliabilityStats struct {
	TotalJobs      int             `json:"total_jobs"`
	SuccessfulJobs int             `json:"successful_jobs"`
	FlakyJobs      int             `json:"flaky_jobs"`
	FailedJobs     int             `json:"failed_jobs"`
	SuccessRate    float64         `json:"success_rate"`
	FlakinessRate  float64         `json:"flakiness_rate"`
	FailureRate    float64         `json:"failure_rate"`
	TotalRetries   int             `json:"total_retries"`
	TopFlakyJobs   []FlakyJobStat  `json:"top_flaky_jobs"`
	TopFailingJobs []FailedJobStat `json:"top_failing_jobs"`
}

type CostStats struct {
	TotalCost             float64 `json:"total_cost"`
	TotalTokensPrompt     float64 `json:"total_tokens_prompt"`
	TotalTokensCompletion float64 `json:"total_tokens_completion"`
	TotalJobs             int     `json:"total_jobs"`
}

type CostByTag struct {
	Tag              string  `json:"tag"`
	Cost             float64 `json:"cost"`
	TokensPrompt     float64 `json:"tokens_prompt"`
	TokensCompletion float64 `json:"tokens_completion"`
	JobsCount        int     `json:"jobs_count"`
}

type CostByModel struct {
	Model            string  `json:"model"`
	Cost             float64 `json:"cost"`
	TokensPrompt     float64 `json:"tokens_prompt"`
	TokensCompletion float64 `json:"tokens_completion"`
	JobsCount        int     `json:"jobs_count"`
}

type CostStatsResponse struct {
	TotalStats       CostStats              `json:"total_stats"`
	TagStats         []CostByTag            `json:"tag_stats"`
	ModelStats       []CostByModel          `json:"model_stats"`
	TopExpensiveJobs []orchestrator.JobInfo `json:"top_expensive_jobs"`
}

type AgentPerformance struct {
	AgentProvider    string        `json:"agent_provider"`
	AgentModel       string        `json:"agent_model"`
	TotalJobs        int           `json:"total_jobs"`
	SuccessfulJobs   int           `json:"successful_jobs"`
	FailedJobs       int           `json:"failed_jobs"`
	SuccessRate      float64       `json:"success_rate"`
	AverageDuration  time.Duration `json:"average_duration"`
	AverageCost      float64       `json:"average_cost"`
	TotalCost        float64       `json:"total_cost"`
	TotalTokens      float64       `json:"total_tokens"`
}

type AgentStatsResponse struct {
	Agents []AgentPerformance `json:"agents"`
}

type analyzeDurationsMsg struct {
	Stats DurationStats
	Err   error
}

type analyzeReliabilityMsg struct {
	Stats ReliabilityStats
	Err   error
}

type analyzeCostsMsg struct {
	Stats CostStatsResponse
	Err   error
}

type analyzeAgentsMsg struct {
	Stats AgentStatsResponse
	Err   error
}

type simulateMsg struct {
	Report orchestrator.SimulationReport
	Err    error
}

type summaryMsg struct {
	Summary map[string]int
	Err     error
}

type TagInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

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

	// Timeout input field
	timeoutInput textinput.Model

	// Deps input field
	depsInput textinput.Model

	// Env input field
	envInput textinput.Model

	// Tags input field
	tagsInput textinput.Model

	// Agent input field
	agentProviderInput textinput.Model
	agentModelInput    textinput.Model

	// Rename input field
	renameInput textinput.Model

	// Max retries input field
	maxRetriesInput textinput.Model

	// Delete pending inputs
	deletePendingGroupInput textinput.Model
	deletePendingTagInput   textinput.Model
	deletePendingMatchInput textinput.Model

	pauseGroupInput  textinput.Model
	resumeGroupInput textinput.Model

	searchInput        textinput.Model
	searchContextInput textinput.Model

	logFilterInput textinput.Model
	isLogFiltering bool

	explain     string
	compareJobs [2]orchestrator.JobInfo
}

type searchLogsResultMsg struct {
	Output string
	Err    error
}

type searchJobsResultMsg struct {
	Jobs []orchestrator.JobInfo
	Err  error
}

type tickMsg time.Time

type explainMsg struct {
	Explanation string
	Err         error
}

type compareMsg struct {
	Jobs [2]orchestrator.JobInfo
	Err  error
}

type statusMsg struct {
	Status orchestrator.Status
	Jobs   []orchestrator.JobInfo
	Err    error
}

type blockersMsg struct {
	Jobs  []orchestrator.JobInfo
	JobID string
	Err   error
}

type tagsMsg struct {
	Tags []TagInfo
	Err  error
}

type dependentsMsg struct {
	Jobs  []orchestrator.JobInfo
	JobID string
	Err   error
}

type analyzeFailuresMsg struct {
	FailedJobs []orchestrator.JobInfo
	Err        error
}

type criticalPathMsg struct {
	Path          []orchestrator.JobInfo
	TotalDuration time.Duration
	Err           error
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

	case summaryMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewSummary
			m.viewport.SetContent(renderSummary(msg.Summary))
			m.viewport.GotoTop()
		}
		return m, nil

	case searchJobsResultMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewSearchJobsResult
			m.viewport.SetContent(renderJobTable(msg.Jobs, "Search Jobs Results"))
			m.viewport.GotoTop()
		}
		return m, nil

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

	case analyzeDurationsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewAnalyzeDurations
			m.viewport.SetContent(renderAnalyzeDurations(msg.Stats))
			m.viewport.GotoTop()
		}
		return m, nil

	case analyzeReliabilityMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewAnalyzeReliability
			m.viewport.SetContent(renderAnalyzeReliability(msg.Stats))
			m.viewport.GotoTop()
		}
		return m, nil

	case tagsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewTags
			m.viewport.SetContent(renderTags(msg.Tags))
			m.viewport.GotoTop()
		}
		return m, nil

	case blockersMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewBlockers
			m.viewport.SetContent(renderJobTable(msg.Jobs, fmt.Sprintf("Blockers of %s", msg.JobID)))
			m.viewport.GotoTop()
		}
		return m, nil

	case dependentsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewDependents
			m.viewport.SetContent(renderJobTable(msg.Jobs, fmt.Sprintf("Dependents of %s", msg.JobID)))
			m.viewport.GotoTop()
		}
		return m, nil

	case criticalPathMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewCriticalPath
			m.viewport.SetContent(renderCriticalPath(msg.Path, msg.TotalDuration))
			m.viewport.GotoTop()
		}
		return m, nil

	case analyzeFailuresMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewAnalyzeFailures
			m.viewport.SetContent(renderAnalyzeFailures(msg.FailedJobs))
			m.viewport.GotoTop()
		}
		return m, nil

	case analyzeCostsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewAnalyzeCosts
			m.viewport.SetContent(renderAnalyzeCosts(msg.Stats))
			m.viewport.GotoTop()
		}
		return m, nil

	case analyzeAgentsMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewAnalyzeAgents
			m.viewport.SetContent(renderAnalyzeAgents(msg.Stats))
			m.viewport.GotoTop()
		}
		return m, nil

	case compareMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.compareJobs = msg.Jobs
			m.viewState = viewCompare
			m.viewport.SetContent(renderCompare(m.compareJobs[0], m.compareJobs[1]))
			m.viewport.GotoTop()
		}
		return m, nil

	case simulateMsg:
		m.err = msg.Err
		if m.err == nil {
			m.viewState = viewSimulate
			m.viewport.SetContent(renderSimulate(msg.Report))
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
			m.isLogFiltering = false
			m.logFilterInput.SetValue("")
			m.viewport.SetContent("")
			m.viewState = viewLogs
			// Start reading chunks
			cmds = append(cmds, waitForLogChunk(m.logStream))
		}
		return m, tea.Batch(cmds...)

	case logChunkMsg:
		if msg.Chunk != "" {
			m.logs += msg.Chunk
			m.updateFilteredLogs()
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

	case searchLogsResultMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.viewState = viewSearchLogsResult
			m.viewport.SetContent(msg.Output)
			m.viewport.GotoTop()
		}
		return m, nil

	case explainMsg:
		if msg.Err != nil {
			m.err = msg.Err
		} else {
			m.explain = msg.Explanation
			m.viewState = viewExplain
			m.viewport.SetContent(renderExplain(m.explain))
			m.viewport.GotoTop()
		}
		return m, nil

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
	case viewLogs:
		m, cmd = m.updateLogsView(msg)
		cmds = append(cmds, cmd)
	case viewDetails, viewAnalytics, viewTree, viewExplain, viewCompare, viewAnalyzeFailures, viewCriticalPath, viewBlockers, viewDependents, viewTags, viewAnalyzeDurations, viewAnalyzeReliability, viewAnalyzeCosts, viewAnalyzeAgents, viewSummary, viewSimulate:
		m, cmd = m.updateViewport(msg)
		cmds = append(cmds, cmd)
	case viewConfirmation:
		m, cmd = m.updateConfirmation(msg)
		cmds = append(cmds, cmd)
	case viewSubmit:
		m, cmd = m.updateSubmit(msg)
		cmds = append(cmds, cmd)
	case viewTimeoutInput:
		m, cmd = m.updateTimeoutInput(msg)
		cmds = append(cmds, cmd)
	case viewDepsInput:
		m, cmd = m.updateDepsInput(msg)
		cmds = append(cmds, cmd)
	case viewEnvInput:
		m, cmd = m.updateEnvInput(msg)
		cmds = append(cmds, cmd)
	case viewTagsInput:
		m, cmd = m.updateTagsInput(msg)
		cmds = append(cmds, cmd)
	case viewAgentInput:
		m, cmd = m.updateAgentInput(msg)
		cmds = append(cmds, cmd)
	case viewRenameInput:
		m, cmd = m.updateRenameInput(msg)
		cmds = append(cmds, cmd)
	case viewMaxRetriesInput:
		m, cmd = m.updateMaxRetriesInput(msg)
		cmds = append(cmds, cmd)
	case viewDeletePendingGroupInput:
		m, cmd = m.updateDeletePendingGroupInput(msg)
		cmds = append(cmds, cmd)
	case viewDeletePendingTagInput:
		m, cmd = m.updateDeletePendingTagInput(msg)
		cmds = append(cmds, cmd)
	case viewDeletePendingMatchInput:
		m, cmd = m.updateDeletePendingMatchInput(msg)
		cmds = append(cmds, cmd)
	case viewPauseGroupInput:
		m, cmd = m.updatePauseGroupInput(msg)
		cmds = append(cmds, cmd)
	case viewResumeGroupInput:
		m, cmd = m.updateResumeGroupInput(msg)
		cmds = append(cmds, cmd)
	case viewSearchLogsInput:
		m, cmd = m.updateSearchLogsInput(msg)
		cmds = append(cmds, cmd)
	case viewSearchLogsContextInput:
		m, cmd = m.updateSearchLogsContextInput(msg)
		cmds = append(cmds, cmd)
	case viewSearchLogsResult:
		m, cmd = m.updateViewport(msg)
		cmds = append(cmds, cmd)
	case viewSearchJobsInput:
		m, cmd = m.updateSearchJobsInput(msg)
		cmds = append(cmds, cmd)
	case viewSearchJobsResult:
		m, cmd = m.updateViewport(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *DashboardModel) updateTableContent() {
	rows := []table.Row{}

	filterText := m.filterInput.Value()

	// Sort jobs by start time (newest first)
	sort.Slice(m.jobs, func(i, j int) bool {
		return m.jobs[i].StartTime.After(m.jobs[j].StartTime)
	})

	for _, job := range m.jobs {
		if filterText != "" {
			idMatch := utils.ContainsFold(job.ID, filterText)
			summaryMatch := utils.ContainsFold(job.Summary, filterText)
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

		statusDisplay := job.Status
		if job.Progress != nil {
			statusDisplay = fmt.Sprintf("%s (%d%%)", job.Status, *job.Progress)
		}
		if job.StatusMessage != nil {
			statusDisplay = fmt.Sprintf("%s - %s", statusDisplay, *job.StatusMessage)
		}
		statusDisplay = limitString(statusDisplay, 25)

		rows = append(rows, table.Row{
			idDisplay,
			limitString(job.Summary, 40),
			statusDisplay,
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
		case "y":
			var idsToCopy []string
			if len(m.selectedJobs) > 0 {
				for id, isSelected := range m.selectedJobs {
					if isSelected {
						idsToCopy = append(idsToCopy, id)
					}
				}
				sort.Strings(idsToCopy)
			} else {
				selected := m.table.SelectedRow()
				if len(selected) > 0 {
					idsToCopy = append(idsToCopy, getRawID(selected[0]))
				}
			}

			if len(idsToCopy) > 0 {
				strToCopy := strings.Join(idsToCopy, ",")
				err := clipboardWriteAll(strToCopy)
				if err != nil {
					return m, func() tea.Msg { return actionMsg{Err: fmt.Errorf("failed to copy: %v", err)} }
				}
				return m, func() tea.Msg {
					return actionMsg{Message: fmt.Sprintf("Copied %d job ID(s) to clipboard", len(idsToCopy))}
				}
			}
			return m, nil
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
		case "=":
			if len(m.selectedJobs) == 2 {
				var ids []string
				for id := range m.selectedJobs {
					ids = append(ids, id)
				}
				return m, fetchCompareJobs(m.host, ids[0], ids[1])
			}
			m.err = fmt.Errorf("Exactly 2 jobs must be selected to compare")
			return m, nil
		case "A":
			return m, fetchAnalytics(m.host)
		case "ctrl+u":
			return m, fetchSummaryCmd(m.host)
		case "ctrl+o":
			return m, fetchAnalyzeCostsCmd(m.host)
		case "ctrl+a":
			return m, fetchAnalyzeAgentsCmd(m.host)
		case "L":
			return m, fetchTagsCmd(m.host)
		case "ctrl+g":
			m.viewState = viewDeletePendingGroupInput
			m.deletePendingGroupInput.SetValue("")
			m.deletePendingGroupInput.Focus()
			return m, textinput.Blink
		case "ctrl+t":
			m.viewState = viewDeletePendingTagInput
			m.deletePendingTagInput.SetValue("")
			m.deletePendingTagInput.Focus()
			return m, textinput.Blink
		case "ctrl+v":
			m.viewState = viewDeletePendingMatchInput
			m.deletePendingMatchInput.SetValue("")
			m.deletePendingMatchInput.Focus()
			return m, textinput.Blink
		case "b":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				return m, fetchBlockersCmd(m.host, id)
			}
		case "B":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				return m, fetchDependentsCmd(m.host, id)
			}
		case "ctrl+p":
			return m, fetchCriticalPathCmd(m.host)
		case "ctrl+j":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_ctrl_j"
				m.pendingAction = "pause group multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			m.viewState = viewPauseGroupInput
			m.pauseGroupInput.SetValue("")
			m.pauseGroupInput.Focus()
			return m, textinput.Blink
		case "ctrl+k":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_ctrl_k"
				m.pendingAction = "resume group multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			m.viewState = viewResumeGroupInput
			m.resumeGroupInput.SetValue("")
			m.resumeGroupInput.Focus()
			return m, textinput.Blink
		case "ctrl+f":
			return m, fetchAnalyzeFailuresCmd(m.host)
		case "ctrl+d":
			return m, fetchAnalyzeDurationsCmd(m.host)
		case "ctrl+r":
			return m, fetchAnalyzeReliabilityCmd(m.host)
		case "ctrl+s":
			return m, fetchSimulateCmd(m.host)
		case "S":
			m.viewState = viewSearchLogsInput
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			return m, textinput.Blink
		case "J":
			m.viewState = viewSearchJobsInput
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			return m, textinput.Blink
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
		case "F":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_F"
				m.pendingAction = "force complete multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "force complete"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "w":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_w"
				m.pendingAction = "archive multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "archive"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "?":
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				return m, fetchExplanation(m.host, id)
			}
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
		case "ctrl+x":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_ctrl_x"
				m.pendingAction = "cancel downstream multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "cancel downstream"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "C":
			m.pendingJobId = "ALL"
			m.pendingAction = "cancel all"
			m.viewState = viewConfirmation
			return m, nil
		case "H":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_H"
				m.pendingAction = "hold multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "hold"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "U":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_U"
				m.pendingAction = "unhold multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "unhold"
				m.viewState = viewConfirmation
				return m, nil
			}
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
		case "ctrl+y":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_ctrl_y"
				m.pendingAction = "retry downstream multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "retry downstream"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "R":
			m.pendingJobId = "FAILED"
			m.pendingAction = "retry failed"
			m.viewState = viewConfirmation
			return m, nil
		case "delete", "backspace":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_delete_pending"
				m.pendingAction = "delete pending multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "delete pending"
				m.viewState = viewConfirmation
				return m, nil
			}
		case "K":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_K"
				m.pendingAction = "heal multiple"
				m.viewState = viewConfirmation
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				m.pendingJobId = getRawID(selected[0])
				m.pendingAction = "heal"
				m.viewState = viewConfirmation
				return m, nil
			}
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
		case "ctrl+e":
			m.pendingJobId = "ALL_CLEAN"
			m.pendingAction = "clean all"
			m.viewState = viewConfirmation
			return m, nil
		case "T":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_timeout"
				m.viewState = viewTimeoutInput
				m.timeoutInput.SetValue("")
				m.timeoutInput.Focus()
				return m, textinput.Blink
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				m.pendingJobId = id
				m.viewState = viewTimeoutInput
				m.timeoutInput.SetValue("")
				m.timeoutInput.Focus()
				return m, textinput.Blink
			}
		case "D":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_deps"
				m.viewState = viewDepsInput
				m.depsInput.SetValue("")
				m.depsInput.Focus()
				return m, textinput.Blink
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.pendingJobId = id
						m.viewState = viewDepsInput
						m.depsInput.SetValue(strings.Join(job.WorkItem.DependsOn, ", "))
						m.depsInput.Focus()
						return m, textinput.Blink
					}
				}
			}
		case "E":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_env"
				m.viewState = viewEnvInput
				m.envInput.SetValue("")
				m.envInput.Focus()
				return m, textinput.Blink
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.pendingJobId = id
						m.viewState = viewEnvInput

						var envPairs []string
						var keys []string
						for k := range job.WorkItem.EnvVars {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, job.WorkItem.EnvVars[k]))
						}

						m.envInput.SetValue(strings.Join(envPairs, ", "))
						m.envInput.Focus()
						return m, textinput.Blink
					}
				}
			}
		case "G":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_tags"
				m.viewState = viewTagsInput
				m.tagsInput.SetValue("")
				m.tagsInput.Focus()
				return m, textinput.Blink
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.pendingJobId = id
						m.viewState = viewTagsInput
						m.tagsInput.SetValue(strings.Join(job.WorkItem.Tags, ", "))
						m.tagsInput.Focus()
						return m, textinput.Blink
					}
				}
			}
		case "M":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_agent"
				m.viewState = viewAgentInput
				m.focusedInput = 0
				m.agentProviderInput.SetValue("")
				m.agentModelInput.SetValue("")
				m.agentProviderInput.Focus()
				m.agentModelInput.Blur()
				return m, textinput.Blink
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.pendingJobId = id
						m.viewState = viewAgentInput
						m.focusedInput = 0
						m.agentProviderInput.SetValue(job.WorkItem.AgentProvider)
						m.agentModelInput.SetValue(job.WorkItem.AgentModel)
						m.agentProviderInput.Focus()
						m.agentModelInput.Blur()
						return m, textinput.Blink
					}
				}
			}
		case "N":
			if len(m.selectedJobs) > 0 {
				m.err = fmt.Errorf("Cannot rename multiple jobs")
				return m, nil
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.pendingJobId = id
						m.viewState = viewRenameInput
						m.renameInput.SetValue(id)
						m.renameInput.Focus()
						return m, textinput.Blink
					}
				}
			}
		case "Z":
			if len(m.selectedJobs) > 0 {
				m.pendingJobId = "MULTIPLE_max_retries"
				m.viewState = viewMaxRetriesInput
				m.maxRetriesInput.SetValue("")
				m.maxRetriesInput.Focus()
				return m, textinput.Blink
			}
			selected := m.table.SelectedRow()
			if len(selected) > 0 {
				id := getRawID(selected[0])
				for _, job := range m.jobs {
					if job.ID == id {
						m.pendingJobId = id
						m.viewState = viewMaxRetriesInput
						if job.WorkItem.MaxRetries != nil {
							m.maxRetriesInput.SetValue(fmt.Sprintf("%d", *job.WorkItem.MaxRetries))
						} else {
							m.maxRetriesInput.SetValue("")
						}
						m.maxRetriesInput.Focus()
						return m, textinput.Blink
					}
				}
			}
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
						m.inputs[3].SetValue(job.WorkItem.ConcurrencyGroup)
						m.inputs[4].SetValue(fmt.Sprintf("%t", job.WorkItem.CancelInProgress))
						m.inputs[5].SetValue(strings.Join(job.WorkItem.Tags, ","))
						m.inputs[6].SetValue(job.WorkItem.AgentProvider)
						m.inputs[7].SetValue(job.WorkItem.AgentModel)
						if job.WorkItem.MaxRetries != nil {
							m.inputs[8].SetValue(fmt.Sprintf("%d", *job.WorkItem.MaxRetries))
						} else {
							m.inputs[8].SetValue("")
						}
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

				processedGroups := make(map[string]bool)
				for id := range m.selectedJobs {
					switch m.pendingAction {
					case "cancel multiple":
						cmds = append(cmds, cancelJob(m.host, id))
					case "cancel downstream multiple":
						cmds = append(cmds, cancelJobDownstream(m.host, id))
					case "force complete multiple":
						cmds = append(cmds, forceCompleteJobCmd(m.host, id))
					case "purge multiple":
						cmds = append(cmds, purgeJobCmd(m.host, id))
					case "retry multiple":
						cmds = append(cmds, retryJob(m.host, id))
					case "retry downstream multiple":
						cmds = append(cmds, retryJobDownstream(m.host, id))
					case "heal multiple":
						cmds = append(cmds, healJobCmd(m.host, id))
					case "approve multiple":
						cmds = append(cmds, approveJobCmd(m.host, id))
					case "hold multiple":
						cmds = append(cmds, holdJobCmd(m.host, id))
					case "unhold multiple":
						cmds = append(cmds, unholdJobCmd(m.host, id))
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
					case "delete pending multiple":
						cmds = append(cmds, deletePendingCmd(m.host, id))
					case "pause group multiple":
						for _, job := range m.jobs {
							if job.ID == id && job.WorkItem.ConcurrencyGroup != "" {
								if !processedGroups[job.WorkItem.ConcurrencyGroup] {
									processedGroups[job.WorkItem.ConcurrencyGroup] = true
									cmds = append(cmds, pauseGroupCmd(m.host, job.WorkItem.ConcurrencyGroup))
								}
								break
							}
						}
					case "resume group multiple":
						for _, job := range m.jobs {
							if job.ID == id && job.WorkItem.ConcurrencyGroup != "" {
								if !processedGroups[job.WorkItem.ConcurrencyGroup] {
									processedGroups[job.WorkItem.ConcurrencyGroup] = true
									cmds = append(cmds, resumeGroupCmd(m.host, job.WorkItem.ConcurrencyGroup))
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
			} else if m.pendingAction == "cancel downstream" {
				cmd = cancelJobDownstream(m.host, m.pendingJobId)
			} else if m.pendingAction == "force complete" {
				cmd = forceCompleteJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "purge" {
				cmd = purgeJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "cancel all" {
				cmd = cancelAllJobs(m.host)
			} else if m.pendingAction == "retry" {
				cmd = retryJob(m.host, m.pendingJobId)
			} else if m.pendingAction == "retry downstream" {
				cmd = retryJobDownstream(m.host, m.pendingJobId)
			} else if m.pendingAction == "retry failed" {
				cmd = retryFailedJobs(m.host)
			} else if m.pendingAction == "heal" {
				cmd = healJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "clear history" {
				cmd = clearHistory(m.host)
			} else if m.pendingAction == "clear pending" {
				cmd = clearPending(m.host)
			} else if m.pendingAction == "clean all" {
				cmd = cleanAllCmd(m.host)
			} else if m.pendingAction == "hold" {
				cmd = holdJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "unhold" {
				cmd = unholdJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "archive" {
				cmd = archiveJobCmd(m.host, m.pendingJobId)
			} else if m.pendingAction == "archive multiple" {
				cmd = archiveBulkJobsCmd(m.host, m.selectedJobs)
			} else if m.pendingAction == "delete pending" {
				cmd = deletePendingCmd(m.host, m.pendingJobId)
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
			concurrencyGroup := m.inputs[3].Value()
			cancelInProgressStr := strings.ToLower(strings.TrimSpace(m.inputs[4].Value()))
			tagsStr := m.inputs[5].Value()
			agentProvider := m.inputs[6].Value()
			agentModel := m.inputs[7].Value()
			maxRetriesStr := m.inputs[8].Value()
			description := m.textarea.Value()

			cancelInProgress := false
			if cancelInProgressStr == "true" || cancelInProgressStr == "t" || cancelInProgressStr == "yes" || cancelInProgressStr == "y" || cancelInProgressStr == "1" {
				cancelInProgress = true
			}

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

			var tags []string
			if tagsStr != "" {
				parts := strings.Split(tagsStr, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						tags = append(tags, trimmed)
					}
				}
			}

			var maxRetries *int
			if maxRetriesStr != "" {
				var val int
				_, err := fmt.Sscanf(maxRetriesStr, "%d", &val)
				if err == nil && val >= 0 {
					maxRetries = &val
				}
			}

			if summary != "" && repoUrl != "" {
				m.viewState = viewMain
				return m, submitJobCmd(m.host, summary, repoUrl, description, dependsOn, tags, concurrencyGroup, cancelInProgress, agentProvider, agentModel, maxRetries)
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

func (m DashboardModel) updateTimeoutInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.timeoutInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "enter":
			val := m.timeoutInput.Value()
			id := m.pendingJobId
			m.viewState = viewMain
			m.timeoutInput.Blur()
			m.pendingJobId = ""

			if val != "" {
				if id == "MULTIPLE_timeout" && len(m.selectedJobs) > 0 {
					var cmds []tea.Cmd
					for jobId := range m.selectedJobs {
						cmds = append(cmds, updateTimeoutCmd(m.host, jobId, val))
					}
					m.selectedJobs = make(map[string]bool)
					m.updateTableContent()
					return m, tea.Batch(cmds...)
				}

				return m, updateTimeoutCmd(m.host, id, val)
			}
			return m, nil
		}
	}
	m.timeoutInput, cmd = m.timeoutInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateDepsInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.depsInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "enter":
			val := m.depsInput.Value()
			id := m.pendingJobId
			m.viewState = viewMain
			m.depsInput.Blur()
			m.pendingJobId = ""

			var deps []string
			if val != "" {
				parts := strings.Split(val, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						deps = append(deps, trimmed)
					}
				}
			}

			if id == "MULTIPLE_deps" && len(m.selectedJobs) > 0 {
				var cmds []tea.Cmd
				for jobId := range m.selectedJobs {
					cmds = append(cmds, updateDependenciesCmd(m.host, jobId, deps))
				}
				m.selectedJobs = make(map[string]bool)
				m.updateTableContent()
				return m, tea.Batch(cmds...)
			}

			return m, updateDependenciesCmd(m.host, id, deps)
		}
	}
	m.depsInput, cmd = m.depsInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateEnvInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.envInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "enter":
			val := m.envInput.Value()
			id := m.pendingJobId
			m.viewState = viewMain
			m.envInput.Blur()
			m.pendingJobId = ""

			env := make(map[string]string)
			if val != "" {
				parts := strings.Split(val, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						kv := strings.SplitN(trimmed, "=", 2)
						if len(kv) == 2 {
							env[kv[0]] = kv[1]
						}
					}
				}
			}

			if id == "MULTIPLE_env" && len(m.selectedJobs) > 0 {
				var cmds []tea.Cmd
				for jobId := range m.selectedJobs {
					cmds = append(cmds, updateEnvCmd(m.host, jobId, env))
				}
				m.selectedJobs = make(map[string]bool)
				m.updateTableContent()
				return m, tea.Batch(cmds...)
			}

			return m, updateEnvCmd(m.host, id, env)
		}
	}
	m.envInput, cmd = m.envInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateRenameInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.renameInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.renameInput.Value())
			id := m.pendingJobId
			m.viewState = viewMain
			m.renameInput.Blur()
			m.pendingJobId = ""

			if val == "" {
				m.err = fmt.Errorf("New ID cannot be empty")
				return m, nil
			}

			return m, updateRenameCmd(m.host, id, val)
		}
	}
	m.renameInput, cmd = m.renameInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateMaxRetriesInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.maxRetriesInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "enter":
			valStr := strings.TrimSpace(m.maxRetriesInput.Value())
			id := m.pendingJobId
			m.viewState = viewMain
			m.maxRetriesInput.Blur()
			m.pendingJobId = ""

			if valStr == "" {
				m.err = fmt.Errorf("Max retries cannot be empty")
				return m, nil
			}

			var val int
			_, err := fmt.Sscanf(valStr, "%d", &val)
			if err != nil || val < 0 {
				m.err = fmt.Errorf("Invalid max retries value: must be a non-negative integer")
				return m, nil
			}

			if id == "MULTIPLE_max_retries" && len(m.selectedJobs) > 0 {
				var cmds []tea.Cmd
				for jobId := range m.selectedJobs {
					cmds = append(cmds, updateMaxRetriesCmd(m.host, jobId, val))
				}
				m.selectedJobs = make(map[string]bool)
				m.updateTableContent()
				return m, tea.Batch(cmds...)
			}

			return m, updateMaxRetriesCmd(m.host, id, val)
		}
	}
	m.maxRetriesInput, cmd = m.maxRetriesInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateTagsInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.tagsInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "enter":
			val := m.tagsInput.Value()
			id := m.pendingJobId
			m.viewState = viewMain
			m.tagsInput.Blur()
			m.pendingJobId = ""

			var tags []string
			if val != "" {
				parts := strings.Split(val, ",")
				for _, p := range parts {
					trimmed := strings.TrimSpace(p)
					if trimmed != "" {
						tags = append(tags, trimmed)
					}
				}
			}

			if id == "MULTIPLE_tags" && len(m.selectedJobs) > 0 {
				var cmds []tea.Cmd
				for jobId := range m.selectedJobs {
					cmds = append(cmds, updateTagsCmd(m.host, jobId, tags))
				}
				m.selectedJobs = make(map[string]bool)
				m.updateTableContent()
				return m, tea.Batch(cmds...)
			}

			return m, updateTagsCmd(m.host, id, tags)
		}
	}
	m.tagsInput, cmd = m.tagsInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateAgentInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.agentProviderInput.Blur()
			m.agentModelInput.Blur()
			m.pendingJobId = ""
			return m, nil
		case "tab", "shift+tab", "up", "down":
			if m.focusedInput == 0 {
				m.focusedInput = 1
				m.agentProviderInput.Blur()
				m.agentModelInput.Focus()
			} else {
				m.focusedInput = 0
				m.agentModelInput.Blur()
				m.agentProviderInput.Focus()
			}
			return m, nil
		case "enter":
			providerVal := m.agentProviderInput.Value()
			modelVal := m.agentModelInput.Value()
			id := m.pendingJobId
			m.viewState = viewMain
			m.agentProviderInput.Blur()
			m.agentModelInput.Blur()
			m.pendingJobId = ""

			if id == "MULTIPLE_agent" && len(m.selectedJobs) > 0 {
				for jobId := range m.selectedJobs {
					cmds = append(cmds, updateAgentCmd(m.host, jobId, providerVal, modelVal))
				}
				m.selectedJobs = make(map[string]bool)
				m.updateTableContent()
				return m, tea.Batch(cmds...)
			}

			return m, updateAgentCmd(m.host, id, providerVal, modelVal)
		}
	}

	var cmd tea.Cmd
	if m.focusedInput == 0 {
		m.agentProviderInput, cmd = m.agentProviderInput.Update(msg)
	} else {
		m.agentModelInput, cmd = m.agentModelInput.Update(msg)
	}
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m DashboardModel) updateDeletePendingGroupInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.deletePendingGroupInput.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.deletePendingGroupInput.Value())
			m.viewState = viewMain
			m.deletePendingGroupInput.Blur()
			if val == "" {
				m.err = fmt.Errorf("Concurrency group cannot be empty")
				return m, nil
			}
			return m, deletePendingBulkCmd(m.host, "group", val)
		}
	}
	m.deletePendingGroupInput, cmd = m.deletePendingGroupInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateDeletePendingTagInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.deletePendingTagInput.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.deletePendingTagInput.Value())
			m.viewState = viewMain
			m.deletePendingTagInput.Blur()
			if val == "" {
				m.err = fmt.Errorf("Tag cannot be empty")
				return m, nil
			}
			return m, deletePendingBulkCmd(m.host, "tag", val)
		}
	}
	m.deletePendingTagInput, cmd = m.deletePendingTagInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateDeletePendingMatchInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.deletePendingMatchInput.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.deletePendingMatchInput.Value())
			m.viewState = viewMain
			m.deletePendingMatchInput.Blur()
			if val == "" {
				m.err = fmt.Errorf("Match regex cannot be empty")
				return m, nil
			}
			return m, deletePendingBulkCmd(m.host, "match", val)
		}
	}
	m.deletePendingMatchInput, cmd = m.deletePendingMatchInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateSearchJobsInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.viewState = viewMain
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			return m, nil
		case "enter":
			val := m.searchInput.Value()
			if val == "" {
				return m, nil
			}
			m.searchInput.Blur()
			return m, searchJobsCmd(m.host, val)
		}
	}
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateSearchLogsContextInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.viewState = viewMain
			m.searchInput.SetValue("")
			m.searchContextInput.SetValue("")
			m.searchContextInput.Blur()
			return m, nil
		case "enter":
			val := m.searchInput.Value()
			ctxVal := m.searchContextInput.Value()
			m.searchContextInput.Blur()
			return m, searchLogsCmd(m.host, val, ctxVal)
		}
	}
	m.searchContextInput, cmd = m.searchContextInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateSearchLogsInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.viewState = viewMain
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			return m, nil
		case "enter":
			val := m.searchInput.Value()
			if val == "" {
				return m, nil
			}
			m.searchInput.Blur()
			m.viewState = viewSearchLogsContextInput
			m.searchContextInput.SetValue("")
			m.searchContextInput.Focus()
			return m, textinput.Blink
		}
	}
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *DashboardModel) updateFilteredLogs() {
	if m.logFilterInput.Value() == "" {
		m.viewport.SetContent(m.logs)
		m.viewport.GotoBottom()
		return
	}

	filterText := m.logFilterInput.Value()
	var filtered []string

	lines := strings.Split(m.logs, "\n")
	for _, line := range lines {
		if utils.ContainsFold(line, filterText) {
			filtered = append(filtered, line)
		}
	}

	m.viewport.SetContent(strings.Join(filtered, "\n"))
	m.viewport.GotoBottom()
}

func (m DashboardModel) updateLogsView(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmds []tea.Cmd

	if m.isLogFiltering {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.isLogFiltering = false
				m.logFilterInput.SetValue("")
				m.logFilterInput.Blur()
				m.updateFilteredLogs()
				return m, nil
			case "enter":
				m.isLogFiltering = false
				m.logFilterInput.Blur()
				return m, nil
			}
			var cmd tea.Cmd
			m.logFilterInput, cmd = m.logFilterInput.Update(msg)
			cmds = append(cmds, cmd)
			m.updateFilteredLogs()
			return m, tea.Batch(cmds...)
		}
	} else {
		// Normal viewport keys + handling '/' to start filtering
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "esc":
				if m.logFilterInput.Value() != "" {
					m.logFilterInput.SetValue("")
					m.updateFilteredLogs()
					return m, nil
				}
				if m.logStream != nil {
					m.logStream.Close()
					m.logStream = nil
				}
				m.viewState = viewMain
				m.isLogFiltering = false
				return m, nil
			case "/":
				m.isLogFiltering = true
				m.logFilterInput.Focus()
				return m, textinput.Blink
			}
		}
	}

	if m.isLogFiltering {
		var cmd tea.Cmd
		m.logFilterInput, cmd = m.logFilterInput.Update(msg)
		cmds = append(cmds, cmd)
	}

	var vpCmd tea.Cmd
	m.viewport, vpCmd = m.viewport.Update(msg)
	cmds = append(cmds, vpCmd)
	return m, tea.Batch(cmds...)
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

		helpView = statusStyle.Render("/: filter | p: pause/resume | d: drain/undrain | f: force poll | F: force complete | P: clear pending | ctrl+g/t/v: clear pending (group/tag/match) | +/-: scale limit | >/<: priority | N: rename | T/D/E/G/M/Z: update | =: compare | h: history | A: analytics | ctrl+u: summary | L: tags | b/B: blockers/deps | ctrl+p: crit path | ctrl+f: failures | ctrl+a: agents | ctrl+o: costs | ctrl+d: durations | ctrl+r: reliability | t: tree | enter: details | l: logs | ?: explain | o: open repo | y: copy ID | a: approve | c: cancel | C: cancel all | ctrl+x: cancel downstream | H/U: hold/unhold | r: retry | R: retry failed | ctrl+y: retry downstream | x: purge | X: clear history | ctrl+e: clean all | e: edit/clone | s: submit | w: archive | q: quit")
	case viewSummary:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewTags:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalyzeDurations:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalyzeReliability:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalyzeCosts:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewSimulate:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalyzeAgents:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewDetails:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewCompare:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalytics:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewAnalyzeFailures:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewCriticalPath:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewBlockers:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewDependents:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewTree:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewDeletePendingGroupInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)
		dialogContent := fmt.Sprintf("Delete Pending by Concurrency Group\n\n%s", m.deletePendingGroupInput.View())
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewDeletePendingTagInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)
		dialogContent := fmt.Sprintf("Delete Pending by Tag\n\n%s", m.deletePendingTagInput.View())
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewDeletePendingMatchInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)
		dialogContent := fmt.Sprintf("Delete Pending by Match Regex\n\n%s", m.deletePendingMatchInput.View())
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewPauseGroupInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)
		dialogContent := fmt.Sprintf("Pause Concurrency Group\n\n%s", m.pauseGroupInput.View())
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewResumeGroupInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)
		dialogContent := fmt.Sprintf("Resume Concurrency Group\n\n%s", m.resumeGroupInput.View())
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewSearchLogsInput:
		inputView := lipgloss.NewStyle().Margin(1, 2).Render(
			titleStyle.Render("Search Logs (Regex):") + "\n" +
				m.searchInput.View() + "\n\n" +
				statusStyle.Render("enter: next | esc: cancel"),
		)
		contentView = baseStyle.Render(inputView)
		helpView = statusStyle.Render("enter: next | esc: cancel")
	case viewSearchLogsContextInput:
		inputView := lipgloss.NewStyle().Margin(1, 2).Render(
			titleStyle.Render("Search Logs (Regex):") + "\n" +
				m.searchInput.View() + "\n\n" +
				titleStyle.Render("Context Lines (optional):") + "\n" +
				m.searchContextInput.View() + "\n\n" +
				statusStyle.Render("enter: search | esc: cancel"),
		)
		contentView = baseStyle.Render(inputView)
		helpView = statusStyle.Render("enter: search | esc: cancel")
	case viewSearchLogsResult:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewSearchJobsInput:
		inputView := lipgloss.NewStyle().Margin(1, 2).Render(
			titleStyle.Render("Search Jobs (Regex):") + "\n" +
				m.searchInput.View() + "\n\n" +
				statusStyle.Render("enter: search | esc: cancel"),
		)
		contentView = baseStyle.Render(inputView)
		helpView = statusStyle.Render("enter: search | esc: cancel")
	case viewSearchJobsResult:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewExplain:
		contentView = baseStyle.Render(m.viewport.View())
		helpView = statusStyle.Render("esc/q: back")
	case viewLogs:
		if m.isLogFiltering || m.logFilterInput.Value() != "" {
			filterView := lipgloss.NewStyle().
				MarginBottom(1).
				Render(m.logFilterInput.View())

			// Reduce height temporarily for the filter input to prevent viewport shifting out of bounds
			originalHeight := m.viewport.Height
			m.viewport.Height -= 2

			contentView = baseStyle.Render(lipgloss.JoinVertical(lipgloss.Left, filterView, m.viewport.View()))

			m.viewport.Height = originalHeight
		} else {
			contentView = baseStyle.Render(m.viewport.View())
		}
		helpView = statusStyle.Render("/: filter logs | esc/q: back | streaming logs...")
	case viewConfirmation:
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
		} else if m.pendingAction == "clean all" {
			dialogMsg = "Are you sure you want to CLEAN ALL?\n(Cancels active jobs, clears pending jobs, and clears history)\n\n(y/Enter: confirm, n/q/Esc: cancel)"
		} else if m.pendingAction == "cancel multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to CANCEL %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "cancel downstream multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to CANCEL %d selected jobs and their downstream dependencies?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "force complete multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to FORCE COMPLETE %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "pause group multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to PAUSE the concurrency groups for %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "resume group multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to RESUME the concurrency groups for %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "purge multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to PURGE %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "retry multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to RETRY %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "retry downstream multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to RETRY %d selected jobs and their downstream dependencies?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "heal multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to HEAL %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "approve multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to APPROVE %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "priority multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to CHANGE PRIORITY for %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "archive multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to ARCHIVE %d selected jobs to bulk_archive.tar.gz?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "archive" {
			dialogMsg = fmt.Sprintf("Are you sure you want to ARCHIVE job %s to %s.tar.gz?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingJobId, m.pendingJobId)
		} else if m.pendingAction == "delete pending" {
			dialogMsg = fmt.Sprintf("Are you sure you want to DELETE PENDING job %s?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingJobId)
		} else if m.pendingAction == "delete pending multiple" {
			dialogMsg = fmt.Sprintf("Are you sure you want to DELETE PENDING for %d selected jobs?\n\n(y/Enter: confirm, n/q/Esc: cancel)", len(m.selectedJobs))
		} else if m.pendingAction == "cancel downstream" {
			dialogMsg = fmt.Sprintf("Are you sure you want to CANCEL job %s and its downstream dependencies?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingJobId)
		} else if m.pendingAction == "retry downstream" {
			dialogMsg = fmt.Sprintf("Are you sure you want to RETRY job %s and its downstream dependencies?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingJobId)
		} else if m.pendingAction == "heal" {
			dialogMsg = fmt.Sprintf("Are you sure you want to HEAL job %s?\n\n(y/Enter: confirm, n/q/Esc: cancel)", m.pendingJobId)
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
		// so for now we just render the dialog inside the container.
		dialogContent := dialogStyle.Render(dialogMsg)

		// If we want to center it nicely we might need more layout logic,
		// but standard center alignment in the container usually works.
		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height+5). // approximate table height
			Align(lipgloss.Center, lipgloss.Center)

		contentView = containerStyle.Render(dialogContent)

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
	case viewTimeoutInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Update Timeout for %s\n\n%s", m.pendingJobId, m.timeoutInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		// Render the dialog centered
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewRenameInput:
		dialogStyle := lipgloss.NewStyle().
			Width(60).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Rename Job %s\n\n%s", m.pendingJobId, m.renameInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		// Render the dialog centered
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewMaxRetriesInput:
		dialogStyle := lipgloss.NewStyle().
			Width(50).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Update Max Retries for %s\n\n%s", m.pendingJobId, m.maxRetriesInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		// Render the dialog centered
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewEnvInput:
		dialogStyle := lipgloss.NewStyle().
			Width(60).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Update Environment for %s\n\n%s", m.pendingJobId, m.envInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		// Render the dialog centered
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewTagsInput:
		dialogStyle := lipgloss.NewStyle().
			Width(60).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Update Tags for %s\n\n%s", m.pendingJobId, m.tagsInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		// Render the dialog centered
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
	case viewAgentInput:
		dialogStyle := lipgloss.NewStyle().
			Width(60).
			Height(7).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Update Agent for %s\n\n%s\n%s", m.pendingJobId, m.agentProviderInput.View(), m.agentModelInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("tab/up/down: switch | enter: confirm | esc: cancel")
	case viewDepsInput:
		dialogStyle := lipgloss.NewStyle().
			Width(60).
			Height(5).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Align(lipgloss.Center, lipgloss.Center).
			Padding(1, 2)

		dialogContent := fmt.Sprintf("Update Dependencies for %s\n\n%s", m.pendingJobId, m.depsInput.View())

		containerStyle := lipgloss.NewStyle().
			Width(m.viewport.Width).
			Height(m.viewport.Height).
			Align(lipgloss.Center, lipgloss.Center)

		// Render the dialog centered
		contentView = containerStyle.Render(dialogStyle.Render(dialogContent))
		helpView = statusStyle.Render("enter: confirm | esc: cancel")
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
		sResp, err := http.Get(host + "/status")
		if err != nil {
			return statusMsg{Err: err}
		}
		defer sResp.Body.Close()

		var status orchestrator.Status
		if err := json.NewDecoder(sResp.Body).Decode(&status); err != nil {
			return statusMsg{Err: err}
		}

		url := host + "/jobs"
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
		resp, err := http.Get(host + "/jobs/" + id)
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

func fetchCompareJobs(host, id1, id2 string) tea.Cmd {
	return func() tea.Msg {
		job1, err := fetchJob(host, id1)
		if err != nil {
			return compareMsg{Err: err}
		}
		job2, err := fetchJob(host, id2)
		if err != nil {
			return compareMsg{Err: err}
		}
		return compareMsg{Jobs: [2]orchestrator.JobInfo{*job1, *job2}}
	}
}

func fetchJob(host, jobID string) (*orchestrator.JobInfo, error) {
	resp, err := http.Get(host + "/jobs/" + jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to orchestrator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &job, nil
}

func fetchExplanation(host, id string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(host + "/jobs/" + id + "/explain")
		if err != nil {
			return explainMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return explainMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var result struct {
			Explanation string `json:"explanation"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return explainMsg{Err: err}
		}

		return explainMsg{Explanation: result.Explanation}
	}
}

func renderExplain(explanation string) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)
	if err != nil {
		return explanation
	}

	out, err := r.Render(explanation)
	if err != nil {
		return explanation
	}

	return out
}

func streamJobLogs(host, id string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(host + "/jobs/" + id + "/logs")
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

func searchJobsCmd(host, query string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/search?q=" + url.QueryEscape(query)
		resp, err := http.Get(urlStr)
		if err != nil {
			return searchJobsResultMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return searchJobsResultMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			return searchJobsResultMsg{Err: err}
		}

		return searchJobsResultMsg{Jobs: jobs}
	}
}

func searchLogsCmd(host, query, contextLines string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/search/logs?q=" + url.QueryEscape(query)
		if contextLines != "" {
			urlStr += "&context=" + url.QueryEscape(contextLines)
		}
		resp, err := http.Get(urlStr)
		if err != nil {
			return searchLogsResultMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return searchLogsResultMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		type LogMatch struct {
			LineNumber int    `json:"line_number"`
			Text       string `json:"text"`
		}
		type JobLogResult struct {
			JobID   string     `json:"job_id"`
			Summary string     `json:"summary"`
			Status  string     `json:"status"`
			Matches []LogMatch `json:"matches"`
		}

		var results []JobLogResult
		if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
			return searchLogsResultMsg{Err: err}
		}

		if len(results) == 0 {
			return searchLogsResultMsg{Output: fmt.Sprintf("No matching logs found for query: %q\n\nPress 'q' or 'esc' to go back, then press 'S' to try a different query.", query)}
		}

		var sb strings.Builder
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
		jobStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
		statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
		lineNumStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
		textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))

		sb.WriteString(titleStyle.Render(fmt.Sprintf("Log Search Results (query: %q)", query)) + "\n\n")

		for _, job := range results {
			sb.WriteString(fmt.Sprintf("Job: %s (%s)\n", jobStyle.Render(job.JobID), statusStyle.Render(job.Status)))
			sb.WriteString(fmt.Sprintf("Summary: %s\n", job.Summary))

			for _, match := range job.Matches {
				sb.WriteString(fmt.Sprintf("  %s %s\n",
					lineNumStyle.Render(fmt.Sprintf("Line %d:", match.LineNumber)),
					textStyle.Render(strings.TrimSpace(match.Text)),
				))
			}
			sb.WriteString("\n")
		}

		return searchLogsResultMsg{Output: sb.String()}
	}
}

func togglePause(host string, isPaused bool) tea.Cmd {
	return func() tea.Msg {
		endpoint := "/pause"
		if isPaused {
			endpoint = "/resume"
		}
		req, err := http.NewRequest(http.MethodPost, host + endpoint, nil)
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
		req, err := http.NewRequest(http.MethodPost, host + endpoint, nil)
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
		req, err := http.NewRequest(http.MethodPost, host + "/poll", nil)
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
		req, err := http.NewRequest(http.MethodPost, host + "/scale", strings.NewReader(reqBody))
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
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/" + id + "/approve", nil)
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
		urlStr := host + "/jobs/" + id + "/priority"
		reqBody := fmt.Sprintf(`{"priority": %d}`, newPriority)

		req, err := http.NewRequest(http.MethodPut, urlStr, strings.NewReader(reqBody))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

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

func updateDependenciesCmd(host, id string, deps []string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/dependencies"

		reqBody := struct {
			DependsOn []string `json:"depends_on"`
		}{
			DependsOn: deps,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Updated dependencies for job %s", id)}
	}
}

func updateEnvCmd(host, id string, env map[string]string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/env"

		reqBody := struct {
			EnvVars map[string]string `json:"env_vars"`
		}{
			EnvVars: env,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Updated environment variables for job %s", id)}
	}
}

func updateRenameCmd(host, id, newID string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/rename"

		reqBody := struct {
			NewID string `json:"new_id"`
		}{
			NewID: newID,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Renamed job %s to %s", id, newID)}
	}
}

func updateMaxRetriesCmd(host, id string, maxRetries int) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/max-retries"

		reqBody := struct {
			MaxRetries int `json:"max_retries"`
		}{
			MaxRetries: maxRetries,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Updated max retries for job %s to %d", id, maxRetries)}
	}
}

func updateTagsCmd(host, id string, tags []string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/tags"

		reqBody := struct {
			Tags []string `json:"tags"`
		}{
			Tags: tags,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Updated tags for job %s", id)}
	}
}

func updateAgentCmd(host, id, provider, model string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/agent"

		reqBody := struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}{
			AgentProvider: provider,
			AgentModel:    model,
		}
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
		if err != nil {
			return actionMsg{Err: err}
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		return actionMsg{Message: fmt.Sprintf("Updated agent for job %s", id)}
	}
}

func updateTimeoutCmd(host, id, newTimeout string) tea.Cmd {
	return func() tea.Msg {
		urlStr := host + "/jobs/" + id + "/timeout"
		reqBody := fmt.Sprintf(`{"timeout": "%s"}`, newTimeout)

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

		return actionMsg{Message: fmt.Sprintf("Updated timeout for job %s to %s", id, newTimeout)}
	}
}

func purgeJobCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host + "/history/" + id, nil)
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

func holdJobCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/" + id + "/hold", nil)
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
		return actionMsg{Message: "Held"}
	}
}

func unholdJobCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/" + id + "/unhold", nil)
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
		return actionMsg{Message: "Unheld"}
	}
}

func forceCompleteJobCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/" + id + "/force-complete", nil)
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
		return actionMsg{Message: "Force Completed"}
	}
}

func cancelJob(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host + "/jobs/" + id, nil)
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

func cancelJobDownstream(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host + "/jobs/" + id + "?downstream=true", nil)
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
		return actionMsg{Message: "Cancelled (Downstream)"}
	}
}

func cancelAllJobs(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host + "/jobs", nil)
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
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/" + id + "/retry", nil)
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

func retryJobDownstream(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/" + id + "/retry?downstream=true", nil)
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
		return actionMsg{Message: "Retry submitted (Downstream)"}
	}
}

func healJobCmd(host, jobID string) tea.Cmd {
	return func() tea.Msg {
		urlStr := fmt.Sprintf("%s/jobs/%s/heal", host, jobID)

		req, err := http.NewRequest(http.MethodPost, urlStr, nil)
		if err != nil {
			return actionMsg{Err: err}
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusAccepted {
			return actionMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var result struct {
			HealedJobID string `json:"healed_job_id"`
		}
		if err := json.Unmarshal(body, &result); err == nil {
			return actionMsg{Message: fmt.Sprintf("Healed job %s", result.HealedJobID)}
		}

		return actionMsg{Message: "Healed"}
	}
}

func clearHistory(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host + "/history", nil)
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

func archiveJobCmd(host, jobID string) tea.Cmd {
	return func() tea.Msg {
		url := host + "/jobs/" + jobID + "/archive"
		resp, err := http.Get(url)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}

		outPath := fmt.Sprintf("%s.tar.gz", jobID)
		f, err := os.Create(outPath)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer f.Close()

		if _, err := io.Copy(f, resp.Body); err != nil {
			return actionMsg{Err: err}
		}

		return actionMsg{Message: fmt.Sprintf("Archived to %s", outPath)}
	}
}

func deletePendingCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host+"/jobs/"+id+"/pending", nil)
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
		return actionMsg{Message: "Pending job deleted"}
	}
}

func (m DashboardModel) updatePauseGroupInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.pauseGroupInput.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.pauseGroupInput.Value())
			m.viewState = viewMain
			m.pauseGroupInput.Blur()
			if val == "" {
				m.err = fmt.Errorf("Concurrency group cannot be empty")
				return m, nil
			}
			return m, pauseGroupCmd(m.host, val)
		}
	}
	m.pauseGroupInput, cmd = m.pauseGroupInput.Update(msg)
	return m, cmd
}

func (m DashboardModel) updateResumeGroupInput(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.viewState = viewMain
			m.resumeGroupInput.Blur()
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.resumeGroupInput.Value())
			m.viewState = viewMain
			m.resumeGroupInput.Blur()
			if val == "" {
				m.err = fmt.Errorf("Concurrency group cannot be empty")
				return m, nil
			}
			return m, resumeGroupCmd(m.host, val)
		}
	}
	m.resumeGroupInput, cmd = m.resumeGroupInput.Update(msg)
	return m, cmd
}

func pauseGroupCmd(host, group string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, host+"/groups/"+group+"/pause", nil)
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
		return actionMsg{Message: "Concurrency group paused"}
	}
}

func resumeGroupCmd(host, group string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodPost, host+"/groups/"+group+"/resume", nil)
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
		return actionMsg{Message: "Concurrency group resumed"}
	}
}

func deletePendingBulkCmd(host, filterType, filterValue string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(host + "/jobs/pending")
		if err != nil {
			return actionMsg{Err: err}
		}
		q := u.Query()
		q.Set(filterType, filterValue)
		u.RawQuery = q.Encode()

		req, err := http.NewRequest(http.MethodDelete, u.String(), nil)
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

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return actionMsg{Err: fmt.Errorf("failed to parse response: %v", err)}
		}
		deleted, ok := result["deleted"].(float64)
		if !ok {
			return actionMsg{Err: fmt.Errorf("invalid response format")}
		}

		return actionMsg{Message: fmt.Sprintf("Deleted %d pending jobs by %s", int(deleted), filterType)}
	}
}

func archiveBulkJobsCmd(host string, selectedJobs map[string]bool) tea.Cmd {
	return func() tea.Msg {
		var ids []string
		for id := range selectedJobs {
			ids = append(ids, id)
		}

		if len(ids) == 0 {
			return actionMsg{Message: "No jobs selected"}
		}

		matchParam := fmt.Sprintf("^(%s)$", strings.Join(ids, "|"))
		urlStr := host + "/jobs/archive/bulk?match=" + url.QueryEscape(matchParam)

		resp, err := http.Get(urlStr)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("status %d", resp.StatusCode)}
		}

		outPath := "bulk_archive.tar.gz"
		f, err := os.Create(outPath)
		if err != nil {
			return actionMsg{Err: err}
		}
		defer f.Close()

		if _, err := io.Copy(f, resp.Body); err != nil {
			return actionMsg{Err: err}
		}

		return actionMsg{Message: fmt.Sprintf("Archived to %s", outPath)}
	}
}

func clearPending(host string) tea.Cmd {
	return func() tea.Msg {
		req, err := http.NewRequest(http.MethodDelete, host + "/pending", nil)
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

func submitJobCmd(host, summary, repoUrl, description string, dependsOn []string, tags []string, concurrencyGroup string, cancelInProgress bool, agentProvider string, agentModel string, maxRetries *int) tea.Cmd {
	return func() tea.Msg {
		// Use a timestamp-based ID or a unique ID.
		// For simplicity, generating an ad-hoc ID
		jobID := fmt.Sprintf("adhoc-%d", time.Now().Unix())

		item := orchestrator.WorkItem{
			ID:               jobID,
			Summary:          summary,
			RepoURL:          repoUrl,
			Description:      description,
			DependsOn:        dependsOn,
			Tags:             tags,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: cancelInProgress,
			AgentProvider:    agentProvider,
			AgentModel:       agentModel,
			MaxRetries:       maxRetries,
		}

		bodyBytes, err := json.Marshal(item)
		if err != nil {
			return actionMsg{Err: err}
		}

		req, err := http.NewRequest(http.MethodPost, host + "/jobs", strings.NewReader(string(bodyBytes)))
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
		req, err := http.NewRequest(http.MethodPost, host + "/jobs/retry-failed", nil)
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
	inputs := make([]textinput.Model, 9)

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

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "group-1"
	inputs[3].Prompt = "Concurrency Group: "
	inputs[3].Width = 50

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "true/false"
	inputs[4].Prompt = "Cancel In Progress: "
	inputs[4].Width = 50

	inputs[5] = textinput.New()
	inputs[5].Placeholder = "bug,feature"
	inputs[5].Prompt = "Tags (comma-separated): "
	inputs[5].Width = 50

	inputs[6] = textinput.New()
	inputs[6].Placeholder = "openrouter"
	inputs[6].Prompt = "Agent Provider: "
	inputs[6].Width = 50

	inputs[7] = textinput.New()
	inputs[7].Placeholder = "openai/gpt-4o-mini"
	inputs[7].Prompt = "Agent Model: "
	inputs[7].Width = 50

	inputs[8] = textinput.New()
	inputs[8].Placeholder = "e.g., 3"
	inputs[8].Prompt = "Max Retries (empty for default): "
	inputs[8].Width = 50

	ta := textarea.New()
	ta.Placeholder = "Detailed description of the issue..."
	ta.SetHeight(10)
	ta.SetWidth(60)

	columns := []table.Column{
		{Title: "ID", Width: 19}, // Increased width for [x] indicator
		{Title: "Summary", Width: 40},
		{Title: "Status", Width: 25},
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

	ti := textinput.New()
	ti.Placeholder = "e.g., 30m or 2h"
	ti.Prompt = "New Timeout: "
	ti.Width = 20

	di := textinput.New()
	di.Placeholder = "e.g., JOB-1, JOB-2"
	di.Prompt = "Dependencies: "
	di.Width = 40

	ei := textinput.New()
	ei.Placeholder = "e.g., KEY=VAL, ANOTHER=VAL2"
	ei.Prompt = "Env Vars: "
	ei.Width = 60

	gi := textinput.New()
	gi.Placeholder = "e.g., bug, feature"
	gi.Prompt = "Tags: "
	gi.Width = 40

	api := textinput.New()
	api.Placeholder = "e.g., openrouter"
	api.Prompt = "Provider: "
	api.Width = 40

	ami := textinput.New()
	ami.Placeholder = "e.g., openai/gpt-4o"
	ami.Prompt = "Model: "
	ami.Width = 40

	ri := textinput.New()
	ri.Placeholder = "NEW-JOB-ID"
	ri.Prompt = "New Job ID: "
	ri.Width = 40

	mri := textinput.New()
	mri.Placeholder = "e.g., 3"
	mri.Prompt = "New Max Retries: "
	mri.Width = 20

		dpgi := textinput.New()
		dpgi.Placeholder = "e.g., group-1"
		dpgi.Prompt = "Concurrency Group: "
		dpgi.Width = 40

		dpti := textinput.New()
		dpti.Placeholder = "e.g., tag-1"
		dpti.Prompt = "Tag: "
		dpti.Width = 40

		dpmi := textinput.New()
		dpmi.Placeholder = "e.g., ^job-.*$"
		dpmi.Prompt = "Match Regex: "
		dpmi.Width = 40

	si := textinput.New()
	si.Placeholder = "e.g., error|panic"
	si.Prompt = "Query: "
	si.Width = 40

	pgi := textinput.New()
	pgi.Placeholder = "Enter concurrency group to pause..."
	rgi := textinput.New()
	rgi.Placeholder = "Enter concurrency group to resume..."
	sci := textinput.New()
	sci.Placeholder = "e.g., 5"
	sci.Prompt = "Context Lines: "
	sci.Width = 40

	lfi := textinput.New()
	lfi.Placeholder = "Filter logs..."
	lfi.Prompt = "/"
	lfi.Width = 40

	return DashboardModel{
		host:         host,
		table:        t,
		viewport:     vp,
		viewState:    viewMain,
		inputs:       inputs,
		textarea:     ta,
		filterInput:  fi,
		isFiltering:  false,
		timeoutInput: ti,
		depsInput:    di,
		envInput:     ei,
		tagsInput:          gi,
		agentProviderInput: api,
		agentModelInput:    ami,
		renameInput:        ri,
		maxRetriesInput:         mri,
		deletePendingGroupInput: dpgi,
		deletePendingTagInput:   dpti,
		deletePendingMatchInput: dpmi,
		pauseGroupInput:         pgi,
		resumeGroupInput:        rgi,
		searchInput:             si,
		searchContextInput:      sci,
		logFilterInput:          lfi,
		selectedJobs:            make(map[string]bool),
	}
}

func StartDashboard(host string, opts ...tea.ProgramOption) error {
	m := NewDashboardModel(host)
	// Enable alt screen for full screen view
	options := append([]tea.ProgramOption{tea.WithAltScreen()}, opts...)
	p := tea.NewProgram(m, options...)

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

func renderJobTable(jobs []orchestrator.JobInfo, title string) string {
	if len(jobs) == 0 {
		return fmt.Sprintf("%s\n\nNo jobs found.\n\nPress 'q' or 'esc' to go back.", title)
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	idCol := lipgloss.NewStyle().Width(15)
	summaryCol := lipgloss.NewStyle().Width(40)
	statusCol := lipgloss.NewStyle().Width(25)
	durationCol := lipgloss.NewStyle().Width(20)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("%s (%d)", title, len(jobs))) + "\n\n")

	sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
		idCol.Render(headerStyle.Render("ID")),
		summaryCol.Render(headerStyle.Render("Summary")),
		statusCol.Render(headerStyle.Render("Status")),
		durationCol.Render(headerStyle.Render("Duration")),
	))

	for _, job := range jobs {
		duration := "N/A"
		if !job.StartTime.IsZero() {
			endTime := job.EndTime
			if endTime.IsZero() {
				endTime = time.Now()
			}
			duration = endTime.Sub(job.StartTime).Round(time.Second).String()
		}

		statusDisplay := job.Status
		if job.Progress != nil {
			statusDisplay = fmt.Sprintf("%s (%d%%)", job.Status, *job.Progress)
		}
		if job.StatusMessage != nil {
			statusDisplay = fmt.Sprintf("%s - %s", statusDisplay, *job.StatusMessage)
		}
		statusDisplay = limitString(statusDisplay, 25)

		sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
			idCol.Render(rowStyle.Render(job.ID)),
			summaryCol.Render(rowStyle.Render(limitString(job.Summary, 38))),
			statusCol.Render(rowStyle.Render(statusDisplay)),
			durationCol.Render(rowStyle.Render(duration)),
		))
	}

	return sb.String()
}

func renderTags(tags []TagInfo) string {
	if len(tags) == 0 {
		return "No tags found across any jobs.\n\nPress 'q' or 'esc' to go back."
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Job Tags (%d)", len(tags))) + "\n\n")

	headerCol1 := headerStyle.Width(30).Render("Tag Name")
	headerCol2 := headerStyle.Width(10).Render("Count")
	sb.WriteString(fmt.Sprintf("%s %s\n", headerCol1, headerCol2))

	for _, tag := range tags {
		rowCol1 := rowStyle.Width(30).Render(limitString(tag.Name, 28))
		rowCol2 := rowStyle.Width(10).Render(fmt.Sprintf("%d", tag.Count))
		sb.WriteString(fmt.Sprintf("%s %s\n", rowCol1, rowCol2))
	}
	sb.WriteString("\nPress 'q' or 'esc' to go back.")

	return sb.String()
}

func fetchBlockersCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(host + "/jobs/" + id + "/blockers")
		if err != nil {
			return blockersMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return blockersMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			return blockersMsg{Err: err}
		}

		return blockersMsg{Jobs: jobs, JobID: id}
	}
}

func fetchSummaryCmd(host string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/summary", host))
		if err != nil {
			return summaryMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return summaryMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var summary map[string]int
		if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
			return summaryMsg{Err: err}
		}

		return summaryMsg{Summary: summary}
	}
}

func renderSummary(summary map[string]int) string {
	if len(summary) == 0 {
		return "No jobs found.\n\nPress 'q' or 'esc' to go back."
	}

	total := 0
	for _, count := range summary {
		total += count
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Width(25).
		Foreground(lipgloss.Color("86"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Job Summary (%d total)", total)) + "\n\n")

	colorMap := map[string]lipgloss.Style{
		"Completed":        lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),  // Green
		"Failed":           lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true), // Red
		"Pending":          lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true), // Orange
		"Pending Approval": lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true), // Orange
		"Spawning":         lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),  // Blue
		"Running":          lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),  // Blue
		"Active":           lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),  // Blue
		"Canceled":         lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true), // Gray
		"Skipped":          lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true), // Gray
		"Retrying":         lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true), // Yellow/Orange
	}

	// Sort statuses to ensure deterministic rendering order
	var statuses []string
	for status := range summary {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)

	for _, status := range statuses {
		count := summary[status]
		lStyle := labelStyle
		if s, ok := colorMap[status]; ok {
			lStyle = s.Width(25)
		}
		sb.WriteString(fmt.Sprintf("%s %s\n", lStyle.Render(status+":"), valueStyle.Render(fmt.Sprintf("%d", count))))
	}
	sb.WriteString("\nPress 'q' or 'esc' to go back.")

	return sb.String()
}

func fetchTagsCmd(host string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("%s/tags", host))
		if err != nil {
			return tagsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return tagsMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var tags []TagInfo
		if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
			return tagsMsg{Err: err}
		}

		return tagsMsg{Tags: tags}
	}
}

func fetchDependentsCmd(host, id string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(host + "/jobs/" + id + "/dependents")
		if err != nil {
			return dependentsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return dependentsMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			return dependentsMsg{Err: err}
		}

		return dependentsMsg{Jobs: jobs, JobID: id}
	}
}

func fetchAnalyzeFailuresCmd(host string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
		if err != nil {
			return analyzeFailuresMsg{Err: err}
		}

		q := u.Query()
		q.Set("state", "all")
		q.Set("status", "Failed")
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return analyzeFailuresMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return analyzeFailuresMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			return analyzeFailuresMsg{Err: err}
		}

		return analyzeFailuresMsg{FailedJobs: jobs}
	}
}

func fetchCriticalPathCmd(host string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
		if err != nil {
			return criticalPathMsg{Err: err}
		}

		q := u.Query()
		q.Set("state", "all")
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return criticalPathMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return criticalPathMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var allJobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&allJobs); err != nil {
			return criticalPathMsg{Err: err}
		}

		path, totalDur := orchestrator.CalculateCriticalPath(allJobs)
		return criticalPathMsg{Path: path, TotalDuration: totalDur}
	}
}

func renderCriticalPath(path []orchestrator.JobInfo, totalDur time.Duration) string {
	if len(path) == 0 {
		return "No jobs available for critical path analysis.\n\nPress 'q' or 'esc' to go back."
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	jobStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	durStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	arrowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Bold(true)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Critical Path Analysis (Total Critical Duration: %s)", totalDur.Round(time.Second))) + "\n")

	for i, j := range path {
		end := j.EndTime
		if end.IsZero() {
			end = time.Now()
		}
		dur := end.Sub(j.StartTime).Round(time.Second)

		sb.WriteString(fmt.Sprintf("%s %s %s\n",
			jobStyle.Render(j.ID),
			durStyle.Render(fmt.Sprintf("[%s]", dur.String())),
			statusStyle.Render(fmt.Sprintf("(%s)", j.Status)),
		))

		if i < len(path)-1 {
			sb.WriteString(arrowStyle.Render("   ↓") + "\n")
		}
	}
	sb.WriteString("\nPress 'q' or 'esc' to go back.")

	return sb.String()
}

func renderAnalyzeFailures(jobs []orchestrator.JobInfo) string {
	if len(jobs) == 0 {
		return "No failed jobs found.\n\nPress 'q' or 'esc' to go back."
	}

	summaryMap := make(map[string][]string) // Summary -> []JobIDs
	for _, job := range jobs {
		summary := strings.TrimSpace(job.Summary)
		if summary == "" {
			summary = "<empty summary>"
		}
		summaryMap[summary] = append(summaryMap[summary], job.ID)
	}

	type summaryGroup struct {
		summary string
		jobIDs  []string
		count   int
	}

	var groups []summaryGroup
	for summary, ids := range summaryMap {
		groups = append(groups, summaryGroup{
			summary: summary,
			jobIDs:  ids,
			count:   len(ids),
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].summary < groups[j].summary
	})

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Failed Jobs Analysis (%d total)", len(jobs))) + "\n\n")

	sb.WriteString(fmt.Sprintf("%-10s %-50s %-40s\n",
		headerStyle.Render("Count"),
		headerStyle.Render("Error Signature (Summary)"),
		headerStyle.Render("Job IDs"),
	))

	for _, g := range groups {
		countStr := fmt.Sprintf("%d", g.count)

		jobIDsStr := strings.Join(g.jobIDs, ", ")
		if len(jobIDsStr) > 38 {
			jobIDsStr = jobIDsStr[:35] + "..."
		}

		sb.WriteString(fmt.Sprintf("%-10s %-50s %-40s\n",
			rowStyle.Render(countStr),
			rowStyle.Render(limitString(g.summary, 48)),
			rowStyle.Render(jobIDsStr),
		))
	}

	return sb.String()
}

func fetchAnalytics(host string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(host + "/analytics")
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
	if a.TotalJobs == 0 {
		return "No analytics available yet.\n\nPress 'q' or 'esc' to go back, then press 's' to submit a new job."
	}

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

	if len(a.TotalMetrics) > 0 {
		s.WriteString("\n" + h1("Total Metrics") + "\n\n")
		for k, v := range a.TotalMetrics {
			s.WriteString(kv(k, fmt.Sprintf("%.2f", v)))
		}
	}

	return s.String()
}

func renderTree(jobs []orchestrator.JobInfo) string {
	if len(jobs) == 0 {
		return "No jobs found.\n\nPress 'q' or 'esc' to go back, then press 's' to submit a new job."
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

	statusDisplay := job.Status
	if job.Progress != nil {
		statusDisplay = fmt.Sprintf("%s (%d%%)", job.Status, *job.Progress)
	}
	if job.StatusMessage != nil {
		statusDisplay = fmt.Sprintf("%s - %s", statusDisplay, *job.StatusMessage)
	}
	s.WriteString(kv("Status", statusDisplay))

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

	if len(job.Metrics) > 0 {
		s.WriteString("\n" + h1("Metrics") + "\n")
		for k, v := range job.Metrics {
			s.WriteString(fmt.Sprintf("  %s=%.2f\n", k, v))
		}
	}

	return s.String()
}
func renderCompare(job1, job2 orchestrator.JobInfo) string {
	var sb strings.Builder

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(30).
		PaddingRight(2)

	diffStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(30).
		PaddingRight(2)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Comparing: %s vs %s", job1.ID, job2.ID)) + "\n")

	printRow := func(label, v1, v2 string) {
		s1 := valueStyle
		s2 := valueStyle
		if v1 != v2 {
			s1 = diffStyle
			s2 = diffStyle
		}
		sb.WriteString(fmt.Sprintf("%s %s | %s\n", headerStyle.Render(label+":"), s1.Render(limitString(v1, 28)), s2.Render(limitString(v2, 28))))
	}

	// Get durations
	dur1 := "N/A"
	if !job1.StartTime.IsZero() {
		if !job1.EndTime.IsZero() {
			dur1 = job1.EndTime.Sub(job1.StartTime).Round(time.Second).String()
		} else {
			dur1 = time.Since(job1.StartTime).Round(time.Second).String() + " (running)"
		}
	}

	dur2 := "N/A"
	if !job2.StartTime.IsZero() {
		if !job2.EndTime.IsZero() {
			dur2 = job2.EndTime.Sub(job2.StartTime).Round(time.Second).String()
		} else {
			dur2 = time.Since(job2.StartTime).Round(time.Second).String() + " (running)"
		}
	}

	printRow("ID", job1.ID, job2.ID)
	printRow("Summary", job1.Summary, job2.Summary)
	printRow("Status", job1.Status, job2.Status)
	printRow("Agent Provider", job1.WorkItem.AgentProvider, job2.WorkItem.AgentProvider)
	printRow("Agent Model", job1.WorkItem.AgentModel, job2.WorkItem.AgentModel)
	printRow("Duration", dur1, dur2)

	// Outputs
	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Outputs ---") + "\n")
	allOutputKeys := make(map[string]bool)
	for k := range job1.Outputs {
		allOutputKeys[k] = true
	}
	for k := range job2.Outputs {
		allOutputKeys[k] = true
	}
	if len(allOutputKeys) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No outputs for either job.") + "\n")
	} else {
		var keys []string
		for k := range allOutputKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v1, ok1 := job1.Outputs[k]
			if !ok1 {
				v1 = "<missing>"
			}
			v2, ok2 := job2.Outputs[k]
			if !ok2 {
				v2 = "<missing>"
			}
			printRow(k, v1, v2)
		}
	}

	// Metrics
	sb.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Metrics ---") + "\n")
	allMetricKeys := make(map[string]bool)
	for k := range job1.Metrics {
		allMetricKeys[k] = true
	}
	for k := range job2.Metrics {
		allMetricKeys[k] = true
	}
	if len(allMetricKeys) == 0 {
		sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No metrics for either job.") + "\n")
	} else {
		var keys []string
		for k := range allMetricKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v1 := "<missing>"
			if val, ok := job1.Metrics[k]; ok {
				v1 = fmt.Sprintf("%.2f", val)
			}
			v2 := "<missing>"
			if val, ok := job2.Metrics[k]; ok {
				v2 = fmt.Sprintf("%.2f", val)
			}
			printRow(k, v1, v2)
		}
	}
	return sb.String()
}
func cleanAllCmd(host string) tea.Cmd {
	return func() tea.Msg {
		// 1. Cancel active jobs
		req, err := http.NewRequest(http.MethodDelete, host+"/jobs", nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("cancel all failed: status %d", resp.StatusCode)}
		}

		// 2. Clear pending jobs
		req, err = http.NewRequest(http.MethodDelete, host+"/pending", nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("clear pending failed: status %d", resp.StatusCode)}
		}

		// 3. Clear history
		req, err = http.NewRequest(http.MethodDelete, host+"/history", nil)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			return actionMsg{Err: err}
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return actionMsg{Err: fmt.Errorf("clear history failed: status %d", resp.StatusCode)}
		}

		return actionMsg{Message: "Clean All: OK"}
	}
}

func fetchAnalyzeDurationsCmd(host string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/durations", host))
		if err != nil {
			return analyzeDurationsMsg{Err: err}
		}

		q := u.Query()
		q.Set("limit", "10") // Default limit to match CLI/WebUI
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return analyzeDurationsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return analyzeDurationsMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var stats DurationStats
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			return analyzeDurationsMsg{Err: err}
		}

		return analyzeDurationsMsg{Stats: stats}
	}
}

func fetchAnalyzeReliabilityCmd(host string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/reliability", host))
		if err != nil {
			return analyzeReliabilityMsg{Err: err}
		}

		q := u.Query()
		q.Set("limit", "10") // Default limit
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return analyzeReliabilityMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return analyzeReliabilityMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var stats ReliabilityStats
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			return analyzeReliabilityMsg{Err: err}
		}

		return analyzeReliabilityMsg{Stats: stats}
	}
}

func fetchAnalyzeCostsCmd(host string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/costs", host))
		if err != nil {
			return analyzeCostsMsg{Err: err}
		}

		q := u.Query()
		q.Set("limit", "10")
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return analyzeCostsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return analyzeCostsMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var stats CostStatsResponse
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			return analyzeCostsMsg{Err: err}
		}

		return analyzeCostsMsg{Stats: stats}
	}
}

func fetchAnalyzeAgentsCmd(host string) tea.Cmd {
	return func() tea.Msg {
		u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/agents", host))
		if err != nil {
			return analyzeAgentsMsg{Err: err}
		}

		q := u.Query()
		q.Set("limit", "10")
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return analyzeAgentsMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return analyzeAgentsMsg{Err: fmt.Errorf("status %d: %s", resp.StatusCode, string(body))}
		}

		var stats AgentStatsResponse
		if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
			return analyzeAgentsMsg{Err: err}
		}

		return analyzeAgentsMsg{Stats: stats}
	}
}

func fetchSimulateCmd(host string) tea.Cmd {
	return func() tea.Msg {
		resp, err := http.Get(fmt.Sprintf("%s/simulate", host))
		if err != nil {
			return simulateMsg{Err: err}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return simulateMsg{Err: fmt.Errorf("HTTP %d", resp.StatusCode)}
		}

		var report orchestrator.SimulationReport
		if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
			return simulateMsg{Err: err}
		}

		return simulateMsg{Report: report}
	}
}

func renderAnalyzeDurations(stats DurationStats) string {
	if stats.TotalJobs == 0 {
		return "No valid completed jobs with duration found.\n\nPress 'q' or 'esc' to go back."
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sb.WriteString(titleStyle.Render(fmt.Sprintf("Duration Analysis (%d valid jobs)", stats.TotalJobs)) + "\n\n")

	printField := func(label, value string) {
		sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value)))
	}

	printField("Total", time.Duration(stats.TotalDuration*float64(time.Millisecond)).Round(time.Second).String())
	printField("Mean", time.Duration(stats.MeanDuration*float64(time.Millisecond)).Round(time.Millisecond).String())
	printField("Median", time.Duration(stats.MedianDuration*float64(time.Millisecond)).Round(time.Millisecond).String())
	printField("Min", time.Duration(stats.MinDuration*float64(time.Millisecond)).Round(time.Millisecond).String())
	printField("Max", time.Duration(stats.MaxDuration*float64(time.Millisecond)).Round(time.Millisecond).String())
	sb.WriteString("\n")

	if len(stats.TagStats) > 0 {
		sb.WriteString(titleStyle.Render("Average Duration by Tag") + "\n\n")
		sb.WriteString(fmt.Sprintf("%-20s %-10s %-20s\n",
			headerStyle.Render("Tag"),
			headerStyle.Render("Count"),
			headerStyle.Render("Mean Duration"),
		))
		tagCol1 := lipgloss.NewStyle().Width(20)
		tagCol2 := lipgloss.NewStyle().Width(10)
		tagCol3 := lipgloss.NewStyle().Width(20)

		for _, ts := range stats.TagStats {
			sb.WriteString(fmt.Sprintf("%s %s %s\n",
				tagCol1.Render(rowStyle.Render(limitString(ts.Tag, 18))),
				tagCol2.Render(fmt.Sprintf("%d", ts.Count)),
				tagCol3.Render(rowStyle.Render(time.Duration(ts.MeanDuration*float64(time.Millisecond)).Round(time.Millisecond).String())),
			))
		}
		sb.WriteString("\n")
	}

	if len(stats.TopSlowest) > 0 {
		sb.WriteString(titleStyle.Render(fmt.Sprintf("Top %d Slowest Jobs", len(stats.TopSlowest))) + "\n\n")

		idCol := lipgloss.NewStyle().Width(15)
		summaryCol := lipgloss.NewStyle().Width(40)
		statusCol := lipgloss.NewStyle().Width(15)
		durationCol := lipgloss.NewStyle().Width(15)

		sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
			idCol.Render(headerStyle.Render("ID")),
			summaryCol.Render(headerStyle.Render("Summary")),
			statusCol.Render(headerStyle.Render("Status")),
			durationCol.Render(headerStyle.Render("Duration")),
		))
		for _, job := range stats.TopSlowest {
			dur := "N/A"
			if !job.StartTime.IsZero() && !job.EndTime.IsZero() {
				dur = job.EndTime.Sub(job.StartTime).Round(time.Millisecond).String()
			}
			sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
				idCol.Render(rowStyle.Render(limitString(job.ID, 13))),
				summaryCol.Render(rowStyle.Render(limitString(job.Summary, 38))),
				statusCol.Render(rowStyle.Render(limitString(job.Status, 13))),
				durationCol.Render(rowStyle.Render(dur)),
			))
		}
	}

	sb.WriteString("\nPress 'q' or 'esc' to go back.")
	return sb.String()
}

func renderSimulate(report orchestrator.SimulationReport) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(25)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sb.WriteString(titleStyle.Render("Pipeline Simulation Report") + "\n\n")

	printField := func(label, value string) {
		sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value)))
	}

	estDuration := time.Duration(report.EstimatedTotalTimeMs) * time.Millisecond
	printField("Estimated Total Time", estDuration.String())
	printField("Jobs Processed", fmt.Sprintf("%d", report.JobsProcessed))
	printField("Total Jobs", fmt.Sprintf("%d", report.TotalJobs))
	printField("Final Bottleneck Job", report.FinalBottleneckJob)
	printField("Deadlocks Detected", fmt.Sprintf("%d", report.Deadlocks))

	sb.WriteString("\nPress 'q' or 'esc' to go back.")

	return sb.String()
}

func renderAnalyzeCosts(stats CostStatsResponse) string {
	if stats.TotalStats.TotalJobs == 0 {
		return "\n  No valid completed jobs with cost data found.\n"
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(25)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		MarginTop(1).
		MarginBottom(1)

	tableHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		PaddingRight(2)

	rowStyle := lipgloss.NewStyle().PaddingRight(2)

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("AI Cost Analysis") + "\n\n")

	printField := func(label, value string) {
		sb.WriteString(fmt.Sprintf("%s %s\n", headerStyle.Render(label+":"), valueStyle.Render(value)))
	}

	printField("Total Evaluated Jobs", fmt.Sprintf("%d", stats.TotalStats.TotalJobs))
	printField("Total Cost", fmt.Sprintf("$%.4f", stats.TotalStats.TotalCost))
	printField("Total Prompt Tokens", fmt.Sprintf("%.0f", stats.TotalStats.TotalTokensPrompt))
	printField("Total Completion Tokens", fmt.Sprintf("%.0f", stats.TotalStats.TotalTokensCompletion))

	if len(stats.TagStats) > 0 {
		sb.WriteString(sectionTitleStyle.Render("Cost by Tag") + "\n")
		sb.WriteString(fmt.Sprintf("%-30s %-15s %-15s\n",
			tableHeaderStyle.Render("Tag"),
			tableHeaderStyle.Render("Jobs"),
			tableHeaderStyle.Render("Total Cost"),
		))
		for _, stat := range stats.TagStats {
			sb.WriteString(fmt.Sprintf("%-30s %-15s %-15s\n",
				rowStyle.Render(stat.Tag),
				rowStyle.Render(fmt.Sprintf("%d", stat.JobsCount)),
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost)),
			))
		}
	}

	if len(stats.ModelStats) > 0 {
		sb.WriteString(sectionTitleStyle.Render("Cost by Model") + "\n")
		sb.WriteString(fmt.Sprintf("%-30s %-15s %-15s\n",
			tableHeaderStyle.Render("Model"),
			tableHeaderStyle.Render("Jobs"),
			tableHeaderStyle.Render("Total Cost"),
		))
		for _, stat := range stats.ModelStats {
			sb.WriteString(fmt.Sprintf("%-30s %-15s %-15s\n",
				rowStyle.Render(stat.Model),
				rowStyle.Render(fmt.Sprintf("%d", stat.JobsCount)),
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost)),
			))
		}
	}

	if len(stats.TopExpensiveJobs) > 0 {
		sb.WriteString(sectionTitleStyle.Render(fmt.Sprintf("Top %d Most Expensive Jobs", len(stats.TopExpensiveJobs))) + "\n")
		sb.WriteString(fmt.Sprintf("%-25s %-40s %-15s\n",
			tableHeaderStyle.Render("ID"),
			tableHeaderStyle.Render("Summary"),
			tableHeaderStyle.Render("Cost"),
		))
		for _, job := range stats.TopExpensiveJobs {
			summary := job.Summary
			if len(summary) > 38 {
				summary = summary[:35] + "..."
			}

			cost := 0.0
			if c, ok := job.Metrics["cost_usd"]; ok {
				cost = c
			}

			sb.WriteString(fmt.Sprintf("%-25s %-40s %-15s\n",
				rowStyle.Render(job.ID),
				rowStyle.Render(summary),
				rowStyle.Render(fmt.Sprintf("$%.4f", cost)),
			))
		}
	}
	sb.WriteString("\n")

	return sb.String()
}

func renderAnalyzeAgents(stats AgentStatsResponse) string {
	if len(stats.Agents) == 0 {
		return "\n  No valid completed jobs with agent data found.\n"
	}

	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#10B981")).
		Padding(0, 1).
		MarginBottom(1)

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		MarginTop(1).
		MarginBottom(1)

	tableHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		PaddingRight(2)

	rowStyle := lipgloss.NewStyle().PaddingRight(2)

	sb.WriteString("\n")
	sb.WriteString(titleStyle.Render("AI Agent Performance Analysis") + "\n")
	sb.WriteString(sectionTitleStyle.Render("Agent Model Metrics") + "\n\n")

	sb.WriteString(fmt.Sprintf("%-15s %-20s %-10s %-12s %-15s %-15s %-12s\n",
		tableHeaderStyle.Render("Provider"),
		tableHeaderStyle.Render("Model"),
		tableHeaderStyle.Render("Jobs"),
		tableHeaderStyle.Render("Success Rate"),
		tableHeaderStyle.Render("Avg Duration"),
		tableHeaderStyle.Render("Avg Cost/Job"),
		tableHeaderStyle.Render("Total Cost"),
	))

	for _, stat := range stats.Agents {
		durationStr := "N/A"
		if stat.AverageDuration > 0 {
			durationStr = stat.AverageDuration.Round(time.Second).String()
		}

		successRateStr := fmt.Sprintf("%.1f%%", stat.SuccessRate*100)

		sb.WriteString(fmt.Sprintf("%-15s %-20s %-10s %-12s %-15s %-15s %-12s\n",
			rowStyle.Render(stat.AgentProvider),
			rowStyle.Render(stat.AgentModel),
			rowStyle.Render(fmt.Sprintf("%d", stat.TotalJobs)),
			rowStyle.Render(successRateStr),
			rowStyle.Render(durationStr),
			rowStyle.Render(fmt.Sprintf("$%.4f", stat.AverageCost)),
			rowStyle.Render(fmt.Sprintf("$%.4f", stat.TotalCost)),
		))
	}
	sb.WriteString("\n")

	return sb.String()
}

func renderAnalyzeReliability(stats ReliabilityStats) string {
	var sb strings.Builder

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	sb.WriteString(titleStyle.Render("Pipeline Reliability Report") + "\n\n")

	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Total Evaluated Jobs:"), valueStyle.Render(fmt.Sprintf("%d", stats.TotalJobs))))
	if stats.TotalJobs > 0 {
		sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Successful Jobs:"), successStyle.Render(fmt.Sprintf("%d (%.2f%%)", stats.SuccessfulJobs, (float64(stats.SuccessfulJobs)/float64(stats.TotalJobs)*100)))))
	} else {
		sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Successful Jobs:"), successStyle.Render(fmt.Sprintf("%d (0.00%%)", stats.SuccessfulJobs))))
	}
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Flaky Jobs:"), warnStyle.Render(fmt.Sprintf("%d (%.2f%%)", stats.FlakyJobs, stats.FlakinessRate))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Failed Jobs:"), errStyle.Render(fmt.Sprintf("%d (%.2f%%)", stats.FailedJobs, stats.FailureRate))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Overall Success Rate (incl. Flaky):"), successStyle.Render(fmt.Sprintf("%.2f%%", stats.SuccessRate))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Total Retries Performed:"), valueStyle.Render(fmt.Sprintf("%d", stats.TotalRetries))))
	sb.WriteString("\n")

	if len(stats.TopFlakyJobs) > 0 {
		sb.WriteString(headerStyle.Render(fmt.Sprintf("Top %d Flaky Jobs (Succeeded eventually, but required retries)", len(stats.TopFlakyJobs))) + "\n")

		sumCol := lipgloss.NewStyle().Width(50)
		occCol := lipgloss.NewStyle().Width(12)
		retCol := lipgloss.NewStyle().Width(15)
		avgCol := lipgloss.NewStyle().Width(12)

		sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
			sumCol.Render(labelStyle.Render("Summary")),
			occCol.Render(labelStyle.Render("Occurrences")),
			retCol.Render(labelStyle.Render("Total Retries")),
			avgCol.Render(labelStyle.Render("Avg Retries")),
		))
		for _, stat := range stats.TopFlakyJobs {
			sb.WriteString(fmt.Sprintf("%s %s %s %s\n",
				sumCol.Render(limitString(stat.Summary, 48)),
				occCol.Render(fmt.Sprintf("%d", stat.Occurrences)),
				retCol.Render(fmt.Sprintf("%d", stat.TotalRetries)),
				avgCol.Render(fmt.Sprintf("%.2f", stat.AvgRetries)),
			))
		}
		sb.WriteString("\n")
	}

	if len(stats.TopFailingJobs) > 0 {
		sb.WriteString(headerStyle.Render(fmt.Sprintf("Top %d Failing Jobs (Failed completely)", len(stats.TopFailingJobs))) + "\n")

		sumCol := lipgloss.NewStyle().Width(50)
		occCol := lipgloss.NewStyle().Width(12)

		sb.WriteString(fmt.Sprintf("%s %s\n",
			sumCol.Render(labelStyle.Render("Summary")),
			occCol.Render(labelStyle.Render("Occurrences")),
		))
		for _, stat := range stats.TopFailingJobs {
			sb.WriteString(fmt.Sprintf("%s %s\n",
				sumCol.Render(limitString(stat.Summary, 48)),
				occCol.Render(fmt.Sprintf("%d", stat.Occurrences)),
			))
		}
		sb.WriteString("\n")
	}

	if len(stats.TopFlakyJobs) == 0 && len(stats.TopFailingJobs) == 0 {
		sb.WriteString(successStyle.Render("Excellent! No flaky or failing jobs detected.") + "\n\n")
	}

	sb.WriteString("Press 'q' or 'esc' to go back.\n")
	return sb.String()
}
