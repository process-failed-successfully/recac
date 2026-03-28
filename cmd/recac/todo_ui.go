package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	// Status bar styles
	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

var todoUICmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TUI for managing TODOs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTodoUI(cmd)
	},
}

func init() {
	todoCmd.AddCommand(todoUICmd)
}

type todoItem struct {
	raw     string
	title   string
	desc    string
	file    string
	line    int
	checked bool
	index   int // 1-based index in TODO.md
}

func (i todoItem) Title() string {
	prefix := "[ ] "
	if i.checked {
		prefix = "[x] "
	}
	return prefix + i.title
}
func (i todoItem) Description() string { return i.desc }
func (i todoItem) FilterValue() string { return i.title + " " + i.desc }

type todoUIModel struct {
	list     list.Model
	viewport viewport.Model
	ready    bool
	err      error
	quitting bool
	msg      string // status message
}

func runTodoUI(cmd *cobra.Command) error {
	// Check if TODO.md exists
	if err := ensureTodoFile(); err != nil {
		return err
	}

	m := newTodoUIModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}
	return nil
}

func newTodoUIModel() todoUIModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "TODOs"
	l.SetShowHelp(true)

	return todoUIModel{
		list:     l,
		viewport: viewport.New(0, 0),
	}
}

func (m todoUIModel) Init() tea.Cmd {
	return loadTasksCmd
}

func (m todoUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Don't intercept keys if filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if i, ok := m.list.SelectedItem().(*todoItem); ok {
				if i.file != "" {
					m.msg = fmt.Sprintf("Solving task %d...", i.index)
					// Show status message
					cmds = append(cmds, m.list.NewStatusMessage(statusMessageStyle("Solving...")))
					return m, tea.Batch(append(cmds, solveTaskCmd(i.index))...)
				} else {
					m.msg = "No file associated with this task."
					return m, m.list.NewStatusMessage(statusMessageStyle("No file info found"))
				}
			}
		case "space":
			if i, ok := m.list.SelectedItem().(*todoItem); ok {
				// Optimistically toggle
				i.checked = !i.checked
				// Trigger update
				return m, toggleTaskCmd(i.index, i.checked)
			}
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		totalWidth := msg.Width - h
		totalHeight := msg.Height - v

		// Split 50/50
		listWidth := totalWidth / 2
		viewWidth := totalWidth - listWidth - 2 // -2 for margin/border if any

		m.list.SetSize(listWidth, totalHeight)
		m.viewport.Width = viewWidth
		m.viewport.Height = totalHeight
		m.ready = true

	case taskListMsg:
		// Reload tasks
		items := make([]list.Item, len(msg))
		for i, item := range msg {
			items[i] = item
		}
		m.list.SetItems(items)
		// Only select first if nothing selected or list was empty
		if len(items) > 0 && len(m.list.Items()) == 0 {
			m.list.Select(0)
		}
		m.updateViewport()

	case solveResultMsg:
		if msg.err != nil {
			m.msg = fmt.Sprintf("Error: %v", msg.err)
			cmds = append(cmds, m.list.NewStatusMessage(statusMessageStyle("Failed")))
		} else {
			m.msg = "Task solved!"
			cmds = append(cmds, m.list.NewStatusMessage(statusMessageStyle("Solved!")))
			// Reload tasks to reflect changes (e.g. marked done)
			cmds = append(cmds, loadTasksCmd)
		}

	case toggleResultMsg:
		if msg.err != nil {
			m.msg = fmt.Sprintf("Error toggling: %v", msg.err)
		} else {
			// Reload to ensure consistency
			cmds = append(cmds, loadTasksCmd)
		}
	}

	// Capture previous selection
	prevItem := m.list.SelectedItem()

	// Update list
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	// Check if selection changed
	if m.list.SelectedItem() != prevItem {
		m.updateViewport()
	}

	return m, tea.Batch(cmds...)
}

func (m *todoUIModel) updateViewport() {
	if i, ok := m.list.SelectedItem().(*todoItem); ok {
		if i.file != "" {
			content, err := os.ReadFile(i.file)
			if err != nil {
				m.viewport.SetContent(fmt.Sprintf("Error reading file: %v", err))
			} else {
				lines := strings.Split(string(content), "\n")
				// Simple line numbering
				var sb strings.Builder
				startLine := i.line - 10
				if startLine < 0 {
					startLine = 0
				}
				endLine := i.line + 10
				if endLine > len(lines) {
					endLine = len(lines)
				}

				for idx := startLine; idx < endLine; idx++ {
					prefix := "  "
					if idx+1 == i.line {
						prefix = "> "
					}
					// Rudimentary syntax highlighting could go here
					sb.WriteString(fmt.Sprintf("%s%d: %s\n", prefix, idx+1, lines[idx]))
				}
				m.viewport.SetContent(sb.String())
			}
		} else {
			m.viewport.SetContent("No file context available.")
		}
	} else {
		m.viewport.SetContent("")
	}
}

func (m todoUIModel) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "Initializing..."
	}

	// Horizontal join of list and viewport
	return docStyle.Render(
		lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), "  ", m.viewport.View()),
	)
}

// Messages
type taskListMsg []*todoItem
type solveResultMsg struct {
	output string
	err    error
}
type toggleResultMsg struct{ err error }

// Commands
func loadTasksCmd() tea.Msg {
	if err := ensureTodoFile(); err != nil {
		return taskListMsg{} // Empty
	}
	lines, err := utils.ReadLines(todoFile)
	if err != nil {
		return taskListMsg{}
	}

	var items []*todoItem
	index := 1
	for _, line := range lines {
		// Only count lines that look like tasks to keep index sync with todo.go logic
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [") {
			item := parseTodoLine(line, index)
			if item != nil {
				items = append(items, item)
			}
			index++
		}
	}
	return taskListMsg(items)
}

func solveTaskCmd(index int) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		// Use multiwriter to print to stdout (hidden in alt screen) or just capture
		err := solveTodoTask(context.Background(), index, &buf)
		return solveResultMsg{output: buf.String(), err: err}
	}
}

func toggleTaskCmd(index int, done bool) tea.Cmd {
	return func() tea.Msg {
		// Reusing todo.go's toggleTaskStatus
		err := toggleTaskStatus(index, done)
		return toggleResultMsg{err: err}
	}
}

// Helper to parse line into todoItem
func parseTodoLine(line string, index int) *todoItem {
	trimmed := strings.TrimSpace(line)
	checked := false
	if strings.HasPrefix(trimmed, "- [x]") {
		checked = true
	} else if !strings.HasPrefix(trimmed, "- [ ]") {
		return nil
	}

	content := ""
	if checked {
		content = strings.TrimPrefix(trimmed, "- [x] ")
	} else {
		content = strings.TrimPrefix(trimmed, "- [ ] ")
	}

	// Try to extract file info: [file:line]
	file := ""
	lineNum := 0

	// Check for [path/to/file:123] pattern
	// Simple split by first ']'
	if strings.HasPrefix(content, "[") {
		endBracket := strings.Index(content, "]")
		if endBracket > 1 {
			fileInfo := content[1:endBracket]
			parts := strings.Split(fileInfo, ":")
			if len(parts) == 2 {
				file = parts[0]
				fmt.Sscanf(parts[1], "%d", &lineNum)
			}
			// Remove the file info from the visible title
			content = strings.TrimSpace(content[endBracket+1:])
		}
	}

	title := content
	desc := ""
	if file != "" {
		desc = fmt.Sprintf("%s:%d", file, lineNum)
	}

	return &todoItem{
		raw:     trimmed,
		title:   title,
		desc:    desc,
		file:    file,
		line:    lineNum,
		checked: checked,
		index:   index,
	}
}
