package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	columnStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.HiddenBorder())
	focusedStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62"))
	boardHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type TicketItem struct {
	ID      string
	Summary string
	Desc    string
	Status  string
}

func (i TicketItem) Title() string       { return fmt.Sprintf("[%s] %s", i.ID, i.Summary) }
func (i TicketItem) Description() string { return i.Desc }
func (i TicketItem) FilterValue() string { return i.Summary }

type BoardModel struct {
	todo       list.Model
	inProgress list.Model
	done       list.Model
	focused    int // 0: todo, 1: inProgress, 2: done
	loaded     bool

	SelectedTicket *TicketItem
	Quitting       bool
	Width          int
	Height         int
}

func NewBoardModel(todos, inProgress, dones []TicketItem) BoardModel {
	// Delegate
	delegate := list.NewDefaultDelegate()

	// Init lists
	lTodo := list.New(itemsToInterface(todos), delegate, 0, 0)
	lTodo.Title = "To Do"
	lTodo.SetShowHelp(false)

	lInProgress := list.New(itemsToInterface(inProgress), delegate, 0, 0)
	lInProgress.Title = "In Progress"
	lInProgress.SetShowHelp(false)

	lDone := list.New(itemsToInterface(dones), delegate, 0, 0)
	lDone.Title = "Done"
	lDone.SetShowHelp(false)

	return BoardModel{
		todo:       lTodo,
		inProgress: lInProgress,
		done:       lDone,
		focused:    0,
	}
}

func itemsToInterface(items []TicketItem) []list.Item {
	res := make([]list.Item, len(items))
	for i, it := range items {
		res[i] = it
	}
	return res
}

func (m BoardModel) Init() tea.Cmd {
	return nil
}

func (m BoardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quitting = true
			return m, tea.Quit
		case "tab", "right", "l":
			m.focused = (m.focused + 1) % 3
			return m, nil
		case "shift+tab", "left", "h":
			m.focused--
			if m.focused < 0 {
				m.focused = 2
			}
			return m, nil
		case "enter":
			if m.focused == 0 {
				if i := m.todo.SelectedItem(); i != nil {
					t := i.(TicketItem)
					m.SelectedTicket = &t
					m.Quitting = true
					return m, tea.Quit
				}
			} else if m.focused == 1 {
				if i := m.inProgress.SelectedItem(); i != nil {
					t := i.(TicketItem)
					m.SelectedTicket = &t
					m.Quitting = true
					return m, tea.Quit
				}
			} else {
				if i := m.done.SelectedItem(); i != nil {
					t := i.(TicketItem)
					m.SelectedTicket = &t
					m.Quitting = true
					return m, tea.Quit
				}
			}
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		columnWidth := msg.Width/3 - 4
		m.todo.SetSize(columnWidth, msg.Height-4)
		m.inProgress.SetSize(columnWidth, msg.Height-4)
		m.done.SetSize(columnWidth, msg.Height-4)
	}

	// Update focused list
	if m.focused == 0 {
		m.todo, cmd = m.todo.Update(msg)
		cmds = append(cmds, cmd)
	} else if m.focused == 1 {
		m.inProgress, cmd = m.inProgress.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.done, cmd = m.done.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m BoardModel) View() string {
	if m.Quitting {
		return ""
	}

	// Styles
	todoView := m.todo.View()
	inProgressView := m.inProgress.View()
	doneView := m.done.View()

	if m.focused == 0 {
		todoView = focusedStyle.Render(todoView)
		inProgressView = columnStyle.Render(inProgressView)
		doneView = columnStyle.Render(doneView)
	} else if m.focused == 1 {
		todoView = columnStyle.Render(todoView)
		inProgressView = focusedStyle.Render(inProgressView)
		doneView = columnStyle.Render(doneView)
	} else {
		todoView = columnStyle.Render(todoView)
		inProgressView = columnStyle.Render(inProgressView)
		doneView = focusedStyle.Render(doneView)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, todoView, inProgressView, doneView)
}
