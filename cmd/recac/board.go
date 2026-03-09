package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"recac/internal/cmdutils"
	"recac/internal/jira"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var boardCmd = &cobra.Command{
	Use:   "board [project-key]",
	Short: "Interactive Jira Kanban Board",
	Long:  `View and manage Jira tickets in an interactive Kanban board TUI.`,
	RunE:  runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
}

func runBoard(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// 1. Get Jira Client (this validates auth)
	client, err := cmdutils.GetJiraClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize Jira client: %w", err)
	}

	// 2. Determine Project Key
	var projectKey string
	if len(args) > 0 {
		projectKey = args[0]
	} else {
		// Try to fetch first project
		pk, err := client.GetFirstProjectKey(ctx)
		if err != nil {
			return fmt.Errorf("failed to get project key (none provided and auto-discovery failed): %w", err)
		}
		projectKey = pk
	}

	// 3. Start TUI
	return runBoardTUIFunc(initialBoardModel(client, projectKey))
}

// runBoardTUIFunc allows mocking the TUI execution in tests
var runBoardTUIFunc = func(m BoardModel) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running board: %w", err)
	}
	return nil
}

/* -------------------------------------------------------------------------
   MODEL
   ------------------------------------------------------------------------- */

type status string

const (
	todo       status = "To Do"
	inProgress status = "In Progress"
	done       status = "Done"
)

type Task struct {
	id          string
	title       string
	description string
	status      status
}

func (t Task) FilterValue() string { return t.title }
func (t Task) Title() string       { return t.id }
func (t Task) Description() string { return t.title }

type BoardModel struct {
	client     *jira.Client
	projectKey string
	cols       []column
	focused    int
	loaded     bool
	err        error
	width      int
	height     int
}

type column struct {
	status status
	list   list.Model
	width  int
}

func initialBoardModel(client *jira.Client, projectKey string) BoardModel {
	return BoardModel{
		client:     client,
		projectKey: projectKey,
		cols: []column{
			newColumn(todo),
			newColumn(inProgress),
			newColumn(done),
		},
		focused: 0,
		loaded:  false,
	}
}

func newColumn(s status) column {
	delegate := list.NewDefaultDelegate()
	l := list.New([]list.Item{}, delegate, 20, 10)
	l.Title = string(s)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetShowTitle(true)
	l.SetShowStatusBar(false)
	// Disable filtering for simplicity in MVP
	l.SetFilteringEnabled(false)
	return column{status: s, list: l}
}

/* -------------------------------------------------------------------------
   INIT & UPDATE
   ------------------------------------------------------------------------- */

func (m BoardModel) Init() tea.Cmd {
	return fetchIssuesCmd(m.client, m.projectKey)
}

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize columns
		colWidth := msg.Width/len(m.cols) - 2 // -2 for margin/border
		for i := range m.cols {
			m.cols[i].list.SetWidth(colWidth)
			m.cols[i].list.SetHeight(msg.Height - 4) // Reserve space for header/footer
			m.cols[i].width = colWidth
		}
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.Left):
			m.focused--
			if m.focused < 0 {
				m.focused = 0
			}
			return m, nil
		case key.Matches(msg, keys.Right):
			m.focused++
			if m.focused >= len(m.cols) {
				m.focused = len(m.cols) - 1
			}
			return m, nil
		case key.Matches(msg, keys.Enter):
			// Move item to next column logic
			// MVP: Toggle status (To Do -> In Progress -> Done -> To Do)
			return m, moveItemCmd(m, 1)
		case key.Matches(msg, keys.Back):
			// Move item to prev column
			return m, moveItemCmd(m, -1)
		case key.Matches(msg, keys.Refresh):
			m.loaded = false
			return m, fetchIssuesCmd(m.client, m.projectKey)
		}

	case issuesMsg:
		// Load issues into columns
		m.loaded = true
		tasks := msg.tasks

		var todoItems []list.Item
		var inProgItems []list.Item
		var doneItems []list.Item

		for _, t := range tasks {
			switch t.status {
			case todo:
				todoItems = append(todoItems, t)
			case inProgress:
				inProgItems = append(inProgItems, t)
			case done:
				doneItems = append(doneItems, t)
			}
		}

		m.cols[0].list.SetItems(todoItems)
		m.cols[1].list.SetItems(inProgItems)
		m.cols[2].list.SetItems(doneItems)
		return m, nil

	case errorMsg:
		m.err = msg.err
		return m, nil

	case moveMsg:
		// Refresh after move
		return m, fetchIssuesCmd(m.client, m.projectKey)
	}

	// Update the focused list
	var cmd tea.Cmd
	m.cols[m.focused].list, cmd = m.cols[m.focused].list.Update(msg)
	return m, cmd
}

/* -------------------------------------------------------------------------
   VIEW
   ------------------------------------------------------------------------- */

func (m BoardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if !m.loaded {
		return "Loading..."
	}

	var cols []string
	for i, col := range m.cols {
		style := columnStyle
		if i == m.focused {
			style = focusedStyle
		}
		cols = append(cols, style.Render(col.list.View()))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, cols...)
}

/* -------------------------------------------------------------------------
   COMMANDS & MESSAGES
   ------------------------------------------------------------------------- */

type issuesMsg struct {
	tasks []Task
}

type errorMsg struct {
	err error
}

type moveMsg struct{}

func fetchIssuesCmd(client *jira.Client, projectKey string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Fetch all issues not done, and recent done issues
		jql := fmt.Sprintf("project = \"%s\" AND statusCategory != Done ORDER BY rank ASC", projectKey)
		activeIssues, err := client.SearchIssues(ctx, jql)
		if err != nil {
			return errorMsg{err}
		}

		jqlDone := fmt.Sprintf("project = \"%s\" AND statusCategory = Done ORDER BY updated DESC", projectKey)
		doneIssues, err := client.SearchIssues(ctx, jqlDone)
		if err != nil {
			return errorMsg{err}
		}

		var tasks []Task

		process := func(issues []map[string]interface{}, s status) {
			for _, issue := range issues {
				key, _ := issue["key"].(string)
				fields, _ := issue["fields"].(map[string]interface{})
				summary, _ := fields["summary"].(string)

				// Refine status if possible, but map to category for columns
				// Actually SearchIssues returns category in fields usually?
				// We used statusCategory in JQL, so we trust the bucket for now.
				// But we need to handle "In Progress" vs "To Do" distinction.

				statusName := "Unknown"
				if statusObj, ok := fields["status"].(map[string]interface{}); ok {
					statusName, _ = statusObj["name"].(string)
					// Map specific status names to our columns if needed
					// For now, if it came from "!= Done" query:
					// If status is "To Do" or "Open" -> To Do
					// If status is "In Progress" -> In Progress
				}

				finalStatus := s
				if s != done {
					// Heuristic mapping
					sn := strings.ToLower(statusName)
					if sn == "in progress" || sn == "review" || sn == "qa" {
						finalStatus = inProgress
					} else {
						finalStatus = todo
					}
				}

				tasks = append(tasks, Task{
					id:      key,
					title:   summary,
					status:  finalStatus,
				})
			}
		}

		// Bucket 1: Active
		// We need to split active into To Do and In Progress
		process(activeIssues, todo) // passing todo as default, logic inside will refine

		// Bucket 2: Done
		process(doneIssues, done)

		return issuesMsg{tasks}
	}
}

func moveItemCmd(m BoardModel, direction int) tea.Cmd {
	return func() tea.Msg {
		col := m.cols[m.focused]
		item := col.list.SelectedItem()
		if item == nil {
			return nil
		}
		task := item.(Task)

		// Determine target status
		// This assumes columns are ordered: To Do -> In Progress -> Done
		var targetStatus string
		if direction > 0 {
			switch task.status {
			case todo:
				targetStatus = "In Progress"
			case inProgress:
				targetStatus = "Done"
			default:
				return nil
			}
		} else {
			switch task.status {
			case inProgress:
				targetStatus = "To Do" // Or "Backlog"
			case done:
				targetStatus = "In Progress"
			default:
				return nil
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Perform transition
		// We use SmartTransition which tries to find transition by name
		if err := m.client.SmartTransition(ctx, task.id, targetStatus); err != nil {
			return errorMsg{fmt.Errorf("failed to move %s to %s: %v", task.id, targetStatus, err)}
		}

		return moveMsg{}
	}
}

/* -------------------------------------------------------------------------
   STYLES & KEYS
   ------------------------------------------------------------------------- */

var (
	columnStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.HiddenBorder())

	focusedStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
)

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Left    key.Binding
	Right   key.Binding
	Enter   key.Binding
	Back    key.Binding
	Refresh key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "move forward"),
	),
	Back: key.NewBinding(
		key.WithKeys("backspace"),
		key.WithHelp("backspace", "move back"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}
