package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TaskStatus represents the state of a task
type TaskStatus int

const (
	TaskPending TaskStatus = iota
	TaskInProgress
	TaskDone
)

// SprintTask represents a task in the sprint board
type SprintTask struct {
	ID     string
	Name   string
	Status TaskStatus
}

// SprintBoardMsg represents messages for updating the board
type SprintBoardMsg struct {
	TaskID string
	Status TaskStatus
}

// SprintBoardModel manages the task board view
type SprintBoardModel struct {
	width    int
	height   int
	ready    bool
	tasks    []SprintTask
	selected int // Currently selected task index
}

// Styles for the task board
var (
	columnStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)

	columnHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFF")).
				Background(lipgloss.Color("#7D56F4")). // Brand Color
				Padding(0, 1).
				MarginBottom(1)

	taskStyle = lipgloss.NewStyle().
			Padding(0, 1).
			MarginBottom(1).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("240"))

	selectedTaskStyle = lipgloss.NewStyle().
				Padding(0, 1).
				MarginBottom(1).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("86")).
				Background(lipgloss.Color("235"))

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#7D56F4")). // Brand Color (matches dashboard)
			Bold(true).
			Padding(0, 1).
			Width(80)
)

// NewSprintBoardModel creates a new sprint board model
func NewSprintBoardModel() SprintBoardModel {
	return SprintBoardModel{
		tasks: make([]SprintTask, 0),
	}
}

// Init initializes the model
func (m SprintBoardModel) Init() tea.Cmd {
	return nil
}

// Update handles messages
func (m SprintBoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected < len(m.tasks)-1 {
				m.selected++
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case SprintBoardMsg:
		// Update task status
		for i := range m.tasks {
			if m.tasks[i].ID == msg.TaskID {
				m.tasks[i].Status = msg.Status
				break
			}
		}
	}

	return m, cmd
}

// AddTask adds a new task to the board
func (m *SprintBoardModel) AddTask(id, name string) {
	m.tasks = append(m.tasks, SprintTask{
		ID:     id,
		Name:   name,
		Status: TaskPending,
	})
}

// View renders the task board
func (m SprintBoardModel) View() string {
	if !m.ready {
		return "Initializing Sprint Board..."
	}

	// Calculate column width
	colWidth := (m.width - 8) / 3 // 3 columns with spacing

	// Group tasks by status
	pending := make([]SprintTask, 0)
	inProgress := make([]SprintTask, 0)
	done := make([]SprintTask, 0)

	for _, task := range m.tasks {
		switch task.Status {
		case TaskPending:
			pending = append(pending, task)
		case TaskInProgress:
			inProgress = append(inProgress, task)
		case TaskDone:
			done = append(done, task)
		}
	}

	// Render columns
	pendingCol := m.renderColumn("Pending", pending, colWidth)
	inProgressCol := m.renderColumn("In Progress", inProgress, colWidth)
	doneCol := m.renderColumn("Done", done, colWidth)

	// Header
	header := headerStyle.Width(m.width).Render("RECAC Sprint Mode - Task Board")

	// Footer
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Width(m.width).
		Render("Press 'q' to quit | Use arrow keys to navigate")

	// Combine columns
	columns := lipgloss.JoinHorizontal(lipgloss.Top, pendingCol, inProgressCol, doneCol)

	// Calculate available height
	headerHeight := 1
	footerHeight := 1
	availableHeight := m.height - headerHeight - footerHeight

	// Wrap columns if needed
	if availableHeight > 0 {
		// Limit column height
		lines := strings.Split(columns, "\n")
		if len(lines) > availableHeight {
			columns = strings.Join(lines[:availableHeight], "\n")
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, columns, footer)
}

// renderColumn renders a single column
func (m SprintBoardModel) renderColumn(title string, tasks []SprintTask, width int) string {
	header := columnHeaderStyle.Width(width - 4).Render(fmt.Sprintf("%s (%d)", title, len(tasks)))

	var taskViews []string
	for i, task := range tasks {
		style := taskStyle
		// Check if this task is selected (simplified - would need better tracking)
		if i == 0 {
			style = selectedTaskStyle
		}

		taskView := style.Width(width - 6).Render(task.Name)
		taskViews = append(taskViews, taskView)
	}

	if len(taskViews) == 0 {
		taskViews = append(taskViews, lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Width(width-6).
			Render("(empty)"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, append([]string{header}, taskViews...)...)

	// Calculate column height based on available space
	headerHeight := 1
	footerHeight := 1
	colHeight := m.height - headerHeight - footerHeight - 2 // Account for padding

	if colHeight < 5 {
		colHeight = 5 // Minimum height
	}

	return columnStyle.Width(width).Height(colHeight).Render(content)
}
