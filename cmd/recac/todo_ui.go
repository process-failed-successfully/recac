package main

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var todoUiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TODO manager",
	Long:  `Launches an interactive TUI for managing TODO items. Supports navigation, toggling status, and solving tasks with AI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTodoUi()
	},
}

func init() {
	todoCmd.AddCommand(todoUiCmd)
}

// todoItem wraps TodoTask to implement list.Item
type todoItem struct {
	TodoTask
}

func (i todoItem) Title() string {
	mark := "[ ]"
	if i.Done {
		mark = "[x]"
	}
	return fmt.Sprintf("%s %s", mark, i.Text)
}

func (i todoItem) Description() string {
	if i.File != "" {
		return fmt.Sprintf("File: %s:%d", i.File, i.Line)
	}
	return "No file context"
}

func (i todoItem) FilterValue() string { return i.Text }

type todoUiModel struct {
	list    list.Model
	spinner spinner.Model
	solving bool
	msg     string
	err     error
	width   int
	height  int
}

func (m todoUiModel) Init() tea.Cmd {
	return nil
}

func (m todoUiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.solving {
			// Allow quitting while solving
			if msg.Type == tea.KeyCtrlC {
				return m, tea.Quit
			}
			return m, nil
		}

		// Don't intercept list keys if filtering
		if m.list.FilterState() == list.Filtering {
			m.list, cmd = m.list.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case " ":
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				err := toggleTaskStatus(i.Index, !i.Done)
				if err != nil {
					return m, m.list.NewStatusMessage(statusMessageStyle(err.Error()))
				}
				// Reload list
				return m, reloadListCmd
			}
		case "enter":
			if i, ok := m.list.SelectedItem().(todoItem); ok {
				if i.File != "" {
					m.solving = true
					m.msg = fmt.Sprintf("Solving: %s", i.Text)
					m.spinner = spinner.New()
					m.spinner.Spinner = spinner.Dot
					m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
					return m, tea.Batch(m.spinner.Tick, solveCmd(i.Index))
				} else {
					return m, m.list.NewStatusMessage(statusMessageStyle("Cannot solve: No file context found."))
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tasksMsg:
		items := make([]list.Item, len(msg))
		for i, t := range msg {
			items[i] = todoItem{t}
		}
		// Remember selection index if possible
		idx := m.list.Index()
		cmd = m.list.SetItems(items)
		if idx < len(items) {
			m.list.Select(idx)
		}
		return m, cmd

	case solveResultMsg:
		m.solving = false
		if msg.Err != nil {
			return m, m.list.NewStatusMessage(statusMessageStyle(fmt.Sprintf("Error: %v", msg.Err)))
		}
		// Task marked done in solveTodoTask, so reload list
		return m, tea.Batch(
			m.list.NewStatusMessage(successMessageStyle("Task solved!")),
			reloadListCmd,
		)

	case spinner.TickMsg:
		if m.solving {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case errMsg:
		return m, m.list.NewStatusMessage(statusMessageStyle(msg.Error()))
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

var docStyle = lipgloss.NewStyle().Margin(1, 2)
var statusMessageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render
var successMessageStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render

func (m todoUiModel) View() string {
	if m.solving {
		return fmt.Sprintf("\n\n   %s %s\n\n", m.spinner.View(), m.msg)
	}
	return docStyle.Render(m.list.View())
}

// Messages

type tasksMsg []TodoTask
type errMsg error

func reloadListCmd() tea.Msg {
	// Delay slightly to ensure file write is flushed
	time.Sleep(50 * time.Millisecond)
	tasks, err := getTodoItems()
	if err != nil {
		return errMsg(err)
	}
	return tasksMsg(tasks)
}

type solveResultMsg struct {
	Err error
	Log string
}

func solveCmd(index int) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		err := solveTodoTask(context.Background(), index, &buf)
		return solveResultMsg{Err: err, Log: buf.String()}
	}
}

func runTodoUi() error {
	tasks, err := getTodoItems()
	if err != nil {
		return err
	}

	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = todoItem{t}
	}

	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "Recac TODO Manager"

	// Helper for keys
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle status")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "solve with AI")),
		}
	}
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "solve")),
		}
	}

	m := todoUiModel{list: l}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
