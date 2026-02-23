package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var docStyle = lipgloss.NewStyle().Margin(1, 2)

var todoUiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TUI for TODO list",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTodoUi()
	},
}

func init() {
	// todoCmd is defined in todo.go
	if todoCmd != nil {
		todoCmd.AddCommand(todoUiCmd)
	}
}

type todoItem struct {
	title       string
	description string
	done        bool
	index       int // 1-based index matching todo.go logic
}

func (i todoItem) Title() string {
	prefix := "[ ]"
	if i.done {
		prefix = "[x]"
	}
	return fmt.Sprintf("%s %s", prefix, i.title)
}
func (i todoItem) Description() string { return i.description }
func (i todoItem) FilterValue() string { return i.title }

type todoModel struct {
	list     list.Model
	input    textinput.Model
	adding   bool
	quitting bool
	err      error
}

func (m todoModel) Init() tea.Cmd {
	return nil
}

func (m todoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.adding {
			switch msg.Type {
			case tea.KeyEnter:
				task := m.input.Value()
				if task != "" {
					if err := appendTaskSilent(task); err != nil {
						m.err = err
					} else {
						m.input.SetValue("")
						m.adding = false
						cmds = append(cmds, m.loadTasks)
					}
				}
			case tea.KeyEsc:
				m.adding = false
				m.input.SetValue("")
			}
			m.input, cmd = m.input.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// List mode keys
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "n":
			m.adding = true
			m.input.Focus()
			return m, textinput.Blink
		case "d", "delete": // List handles delete? No, we handle it.
			if m.list.FilterState() == list.Filtering {
				break // Let list handle input
			}
			if item, ok := m.list.SelectedItem().(todoItem); ok {
				if err := removeTaskSilent(item.index); err != nil {
					m.err = err
				} else {
					cmds = append(cmds, m.loadTasks)
				}
			}
			return m, tea.Batch(cmds...) // Don't pass 'd' to list if we handled it
		case "enter", "space", " ":
			if m.list.FilterState() == list.Filtering {
				break
			}
			if item, ok := m.list.SelectedItem().(todoItem); ok {
				if err := toggleTaskStatus(item.index, !item.done); err != nil {
					m.err = err
				} else {
					cmds = append(cmds, m.loadTasks)
				}
			}
			return m, tea.Batch(cmds...)
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case runMsg:
		cmd = m.list.SetItems(msg)
		return m, cmd
	}

	if !m.adding {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m todoModel) View() string {
	if m.quitting {
		return ""
	}
	if m.adding {
		return fmt.Sprintf(
			"Add a new task:\n\n%s\n\n(esc to cancel, enter to save)",
			m.input.View(),
		)
	}
	return docStyle.Render(m.list.View())
}

func (m todoModel) loadTasks() tea.Msg {
	items, err := loadTodoItems()
	if err != nil {
		return nil // TODO: handle error
	}
	return runMsg(items)
}

type runMsg []list.Item

func runTodoUi() error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	items, err := loadTodoItems()
	if err != nil {
		return err
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Recac TODO"
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new task")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
		}
	}

	ti := textinput.New()
	ti.Placeholder = "New task..."
	ti.CharLimit = 156
	ti.Width = 20

	m := todoModel{
		list:  l,
		input: ti,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

func loadTodoItems() ([]list.Item, error) {
	lines, err := utils.ReadLines(todoFilename)
	if err != nil {
		return nil, err
	}

	var items []list.Item
	index := 1 // Matches 1-based index in todo.go
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		var title string
		var done bool

		if strings.HasPrefix(trimmed, "- [ ]") {
			title = strings.TrimPrefix(trimmed, "- [ ] ")
			done = false
		} else if strings.HasPrefix(trimmed, "- [x]") {
			title = strings.TrimPrefix(trimmed, "- [x] ")
			done = true
		} else {
			// Count non-task lines in index?
			// todo.go listTasks:
			// currentIndex := 1
			// for _, line ...
			//   if prefix ... currentIndex++
			//   else ... (does not increment index)
			// Wait, let's check modifyTask in todo.go

			// modifyTask:
			// currentIndex := 1
			// for _, line ...
			//   if prefix ...
			//      if currentIndex == targetIndex ...
			//      currentIndex++
			//   else ...
			//      (just append)

			// So index only increments for task lines.
			continue
		}

		// Try to parse description from [file:line]
		// Format: [path:line] Keyword: Content
		description := ""
		if strings.HasPrefix(title, "[") {
			end := strings.Index(title, "]")
			if end > 1 {
				description = title[1:end] // extract file:line
				// Clean title? maybe
			}
		}

		items = append(items, todoItem{
			title:       title,
			description: description,
			done:        done,
			index:       index,
		})
		index++
	}
	return items, nil
}

func appendTaskSilent(task string) error {
	if err := ensureTodoFile(); err != nil {
		return err
	}

	f, err := os.OpenFile(todoFilename, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("- [ ] %s\n", task)); err != nil {
		return err
	}
	return nil
}

func removeTaskSilent(targetIndex int) error {
	return modifyTask(targetIndex, func(trimmed string) (string, bool) {
		return "", false
	})
}
