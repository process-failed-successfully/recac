package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	menuTitleStyle      = lipgloss.NewStyle().MarginLeft(2)
	menuPaginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	menuHelpStyle       = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	menuQuitTextStyle   = lipgloss.NewStyle().Margin(1, 0, 2, 4)
)

type MenuItem struct {
	Name, Desc string
}

func (i MenuItem) Title() string       { return i.Name }
func (i MenuItem) Description() string { return i.Desc }
func (i MenuItem) FilterValue() string { return i.Name }

type MenuModel struct {
	list     list.Model
	Selected string
	Quitting bool
}

func NewMenuModel(items []MenuItem) MenuModel {
	lItems := make([]list.Item, len(items))
	for i, item := range items {
		lItems[i] = item
	}

	const defaultWidth = 20
	const listHeight = 14

	l := list.New(lItems, list.NewDefaultDelegate(), defaultWidth, listHeight)
	l.Title = "Recac Command Menu"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = menuTitleStyle
	l.Styles.PaginationStyle = menuPaginationStyle
	l.Styles.HelpStyle = menuHelpStyle

	return MenuModel{list: l}
}

func (m MenuModel) Init() tea.Cmd {
	return nil
}

func (m MenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			m.Quitting = true
			return m, tea.Quit

		case "enter":
			i, ok := m.list.SelectedItem().(MenuItem)
			if ok {
				m.Selected = i.Name
			}
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m MenuModel) View() string {
	if m.Selected != "" {
		return ""
	}
	if m.Quitting {
		return menuQuitTextStyle.Render("Bye!")
	}
	return "\n" + m.list.View()
}
