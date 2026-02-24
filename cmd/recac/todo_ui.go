package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
)

type TodoModel struct {
	list     list.Model
	input    textinput.Model
	adding   bool
	quitting bool
	err      error
}

func NewTodoModel() (TodoModel, error) {
	tasks, err := loadTasks()
	if err != nil {
		return TodoModel{}, err
	}

	items := make([]list.Item, len(tasks))
	for i, t := range tasks {
		items[i] = t
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("205")).BorderForeground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("205")).BorderForeground(lipgloss.Color("205"))

	l := list.New(items, delegate, 0, 0)
	l.Title = "TODO List"

	// Define additional key bindings for help view
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "toggle")),
		}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add task")),
			key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete task")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "toggle status")),
		}
	}

	ti := textinput.New()
	ti.Placeholder = "New task..."
	ti.CharLimit = 100
	ti.Width = 50

	return TodoModel{
		list:  l,
		input: ti,
	}, nil
}

func (m TodoModel) Init() tea.Cmd {
	return nil
}

func (m TodoModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.adding {
			switch msg.String() {
			case "enter":
				taskDesc := m.input.Value()
				if taskDesc != "" {
					// Add task
					newTask := Task{Desc: taskDesc, Done: false}
					cmd = m.list.InsertItem(len(m.list.Items()), newTask)
					cmds = append(cmds, cmd)
					m.input.SetValue("")
					m.adding = false
					// Save changes
					if err := m.save(); err != nil {
						m.err = err
					}
				}
				return m, tea.Batch(cmds...)
			case "esc":
				m.adding = false
				m.input.SetValue("")
				m.input.Blur()
				return m, nil
			}
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		// Don't intercept keys if filtering
		if m.list.FilterState() == list.Filtering {
			break
		}

		// Normal list mode
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "a":
			m.adding = true
			m.input.Focus()
			return m, textinput.Blink
		case "d":
			if len(m.list.Items()) > 0 {
				index := m.list.Index()
				m.list.RemoveItem(index)
				if err := m.save(); err != nil {
					m.err = err
				}
			}
			return m, nil
		case "enter":
			if len(m.list.Items()) > 0 {
				index := m.list.Index()
				item := m.list.Items()[index]
				if task, ok := item.(Task); ok {
					task.Done = !task.Done
					m.list.SetItem(index, task)
					if err := m.save(); err != nil {
						m.err = err
					}
				}
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m TodoModel) View() string {
	if m.quitting {
		return ""
	}
	if m.adding {
		return fmt.Sprintf(
			"\n  Add a task:\n\n%s\n\n  %s",
			m.input.View(),
			"(esc to cancel, enter to save)",
		)
	}
	return docStyle.Render(m.list.View())
}

func (m TodoModel) save() error {
	items := m.list.Items()
	tasks := make([]Task, len(items))
	for i, item := range items {
		if task, ok := item.(Task); ok {
			tasks[i] = task
		}
	}
	return saveTasks(tasks)
}
