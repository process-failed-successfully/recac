package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var todoUiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TUI for managing tasks",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(initialTodoModel())
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
	subtleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dotStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("236")).Render(" • ")
	itemStyle   = lipgloss.NewStyle().PaddingLeft(4)
	doneStyle   = lipgloss.NewStyle().Strikethrough(true).Foreground(lipgloss.Color("243"))
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	titleStyle  = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Padding(0, 1)
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
)

type TodoModel struct {
	tasks    []TodoTask
	cursor   int
	adding   bool
	input    textinput.Model
	err      error
	quitting bool
}

func initialTodoModel() TodoModel {
	ti := textinput.New()
	ti.Placeholder = "New task..."
	ti.CharLimit = 156
	ti.Width = 50

	tasks, _ := loadTasks() // Ignore error for initial load, will retry or show empty

	return TodoModel{
		tasks:  tasks,
		input:  ti,
		cursor: 0,
	}
}

func (m TodoModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m TodoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.adding {
			switch msg.String() {
			case "enter":
				taskDesc := m.input.Value()
				if taskDesc != "" {
					// Add task
					m.tasks = append(m.tasks, TodoTask{Description: taskDesc, Done: false})
					if err := saveTasks(m.tasks); err != nil {
						m.err = err
					}
					m.input.Reset()
					m.adding = false
					m.cursor = len(m.tasks) - 1
				} else {
					m.adding = false
				}
				return m, nil
			case "esc":
				m.adding = false
				m.input.Reset()
				return m, nil
			}
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
		case "enter", " ":
			if len(m.tasks) > 0 {
				m.tasks[m.cursor].Done = !m.tasks[m.cursor].Done
				if err := saveTasks(m.tasks); err != nil {
					m.err = err
				}
			}
		case "d", "x":
			if len(m.tasks) > 0 {
				m.tasks = append(m.tasks[:m.cursor], m.tasks[m.cursor+1:]...)
				if err := saveTasks(m.tasks); err != nil {
					m.err = err
				}
				if m.cursor >= len(m.tasks) && m.cursor > 0 {
					m.cursor--
				}
			}
		case "a":
			m.adding = true
			m.input.Focus()
			return m, textinput.Blink
		}
	}

	return m, cmd
}

func (m TodoModel) View() string {
	if m.quitting {
		return ""
	}

	s := strings.Builder{}
	s.WriteString(titleStyle.Render("TODO List"))
	s.WriteString("\n\n")

	for i, task := range m.tasks {
		cursor := " "
		if m.cursor == i {
			cursor = cursorStyle.Render(">")
		}

		checked := " "
		if task.Done {
			checked = checkStyle.Render("✓")
		}

		desc := task.Description
		if task.Done {
			desc = doneStyle.Render(desc)
		}

		s.WriteString(fmt.Sprintf("%s [%s] %s\n", cursor, checked, desc))
	}

	if len(m.tasks) == 0 && !m.adding {
		s.WriteString(subtleStyle.Render("No tasks. Press 'a' to add one."))
	}

	s.WriteString("\n")

	if m.adding {
		s.WriteString(fmt.Sprintf("Add task: %s", m.input.View()))
	} else {
		s.WriteString(helpStyle.Render("j/k: navigate • space: toggle • a: add • d: delete • q: quit"))
	}

	if m.err != nil {
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return s.String()
}
