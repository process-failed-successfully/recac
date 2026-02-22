package main

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"recac/internal/utils"
)

var todoUiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TODO manager",
	Long:  `Manage your TODO list interactively with a TUI. Navigate with arrow keys, press Enter to toggle status, 's' to solve with AI, 'r' to remove.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(newTodoModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	todoCmd.AddCommand(todoUiCmd)
}

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0A0A0")).
			Padding(0, 1)
)

type todoItem struct {
	raw     string
	content string
	file    string
	line    int
	done    bool
	index   int // 1-based index in TODO.md
}

func (i todoItem) Title() string {
	prefix := "[ ] "
	if i.done {
		prefix = "[x] "
	}
	return prefix + i.content
}

func (i todoItem) Description() string {
	if i.file != "" {
		return fmt.Sprintf("%s:%d", i.file, i.line)
	}
	return "Manual Task"
}

func (i todoItem) FilterValue() string { return i.content }

type todoModel struct {
	list     list.Model
	spinner  spinner.Model
	solving  bool
	err      error
	quitting bool
	width    int
	height   int
}

func newTodoModel() todoModel {
	items := loadTodoItems()

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#25A065")).BorderLeftForeground(lipgloss.Color("#25A065"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("#1C7D4E")).BorderLeftForeground(lipgloss.Color("#25A065"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "Recac TODOs"
	l.Styles.Title = titleStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return todoModel{
		list:    l,
		spinner: s,
	}
}

func (m todoModel) Init() tea.Cmd {
	return nil
}

func (m todoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if m.solving {
			return m, nil // Block input while solving
		}
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				toggleTaskStatus(i.index, !i.done)
				// Reload items to reflect change
				m.list.SetItems(loadTodoItems())
				// Restore selection
				// (Simple approach: selection might reset, but for now ok)
			}

		case "r":
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				removeTask(i.index)
				m.list.SetItems(loadTodoItems())
			}

		case "s":
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				if !i.done && i.file != "" {
					m.solving = true
					return m, tea.Batch(
						m.spinner.Tick,
						solveTaskCmd(i.index),
					)
				}
				m.list.NewStatusMessage(statusMessageStyle.Render("Cannot solve: already done or no file context."))
			}
		}

	case spinner.TickMsg:
		if m.solving {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case solvedMsg:
		m.solving = false
		if msg.err != nil {
			m.list.NewStatusMessage(statusMessageStyle.Render("Error: " + msg.err.Error()))
		} else {
			m.list.NewStatusMessage(statusMessageStyle.Render("Task solved!"))
			m.list.SetItems(loadTodoItems())
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m todoModel) View() string {
	if m.quitting {
		return ""
	}

	if m.solving {
		return fmt.Sprintf("\n %s Solving task with AI agent...\n\n", m.spinner.View())
	}

	return appStyle.Render(m.list.View())
}

// Helpers

type solvedMsg struct {
	err error
}

func solveTaskCmd(index int) tea.Cmd {
	return func() tea.Msg {
		// Use a discarding writer since we show progress via spinner/status
		err := solveTodoTask(context.Background(), index, io.Discard)
		return solvedMsg{err: err}
	}
}

func loadTodoItems() []list.Item {
	ensureTodoFile()
	lines, err := utils.ReadLines(todoFile)
	if err != nil {
		return []list.Item{}
	}

	var items []list.Item
	re := regexp.MustCompile(`\[([^]]+):(\d+)\]`)

	index := 1
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		isTodo := strings.HasPrefix(trimmed, "- [ ]")
		isDone := strings.HasPrefix(trimmed, "- [x]")

		if isTodo || isDone {
			content := ""
			if isTodo {
				content = strings.TrimPrefix(trimmed, "- [ ] ")
			} else {
				content = strings.TrimPrefix(trimmed, "- [x] ")
			}

			// Parse file context if available
			file := ""
			lineNum := 0
			matches := re.FindStringSubmatch(content)
			if len(matches) >= 3 {
				file = matches[1]
				lineNum, _ = strconv.Atoi(matches[2])
				// Clean content to show just the text if desired, or keep raw
				// keeping raw content in Title for now, but maybe strip [file:line]
			}

			items = append(items, todoItem{
				raw:     trimmed,
				content: content,
				file:    file,
				line:    lineNum,
				done:    isDone,
				index:   index,
			})
			index++
		}
	}
	return items
}
