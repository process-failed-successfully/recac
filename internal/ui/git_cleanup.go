package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type BranchStatus string

const (
	StatusMerged   BranchStatus = "merged"
	StatusUnmerged BranchStatus = "unmerged"
	StatusStale    BranchStatus = "stale"
	StatusActive   BranchStatus = "active" // Recent but unmerged
)

// BranchItem represents a git branch in the list.
type BranchItem struct {
	Name       string
	Status     BranchStatus
	LastCommit string
	Author     string
	IsSelected bool
}

func (i BranchItem) Title() string {
	prefix := "[ ]"
	if i.IsSelected {
		prefix = "[x]"
	}
	return fmt.Sprintf("%s %s", prefix, i.Name)
}

func (i BranchItem) Description() string {
	return fmt.Sprintf("%s | %s | %s", i.Status, i.Author, i.LastCommit)
}

func (i BranchItem) FilterValue() string { return i.Name + " " + string(i.Status) + " " + i.Author }

// GitCleanupModel is the Bubble Tea model for the git cleanup tool.
type GitCleanupModel struct {
	list          list.Model
	selectedItems map[string]bool // Map of branch name to selection state
	width         int
	height        int
	quitting      bool
	confirmed     bool

	// Confirmation dialog
	confirming bool
}

// NewGitCleanupModel creates a new git cleanup model.
func NewGitCleanupModel(branches []BranchItem) GitCleanupModel {
	items := make([]list.Item, len(branches))
	for i, b := range branches {
		items[i] = b
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true

	// Custom styles
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("205")).BorderForeground(lipgloss.Color("205"))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color("240"))

	m := GitCleanupModel{
		list:          list.New(items, delegate, 0, 0),
		selectedItems: make(map[string]bool),
	}
	m.list.Title = "Git Branch Cleanup"
	m.list.SetShowHelp(true)

	m.list.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle selection")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "delete selected")),
			key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "select all merged")),
		}
	}
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "delete")),
		}
	}

	return m
}

func (m GitCleanupModel) Init() tea.Cmd {
	return nil
}

func (m GitCleanupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		if m.confirming {
			switch msg.String() {
			case "y", "Y":
				m.confirmed = true
				m.quitting = true
				return m, tea.Quit
			case "n", "N", "esc", "q":
				m.confirming = false
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case " ", "space":
			item := m.list.SelectedItem()
			if item != nil {
				b := item.(BranchItem)
				m.selectedItems[b.Name] = !m.selectedItems[b.Name]
				// Trigger list update to redraw?
				// list.Model doesn't automatically redraw based on external state if items didn't change.
				// We need to update the item in the list.
				idx := m.list.Index()
				b.IsSelected = m.selectedItems[b.Name]
				m.list.SetItem(idx, b)
			}
			return m, nil

		case "enter":
			count := 0
			for _, v := range m.selectedItems {
				if v {
					count++
				}
			}
			if count > 0 {
				m.confirming = true
			}
			return m, nil

		case "a":
			// Select all merged
			items := m.list.Items()
			for i, it := range items {
				b := it.(BranchItem)
				if b.Status == StatusMerged || b.Status == StatusStale {
					m.selectedItems[b.Name] = true
					b.IsSelected = true
					m.list.SetItem(i, b)
				}
			}
			return m, nil
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m GitCleanupModel) View() string {
	if m.confirming {
		count := 0
		for _, v := range m.selectedItems {
			if v {
				count++
			}
		}
		return fmt.Sprintf("\n\n  Are you sure you want to delete %d branches? (y/N)\n\n", count)
	}

	return m.list.View()
}

// GetSelectedBranches returns the list of selected branch names
func (m GitCleanupModel) GetSelectedBranches() []string {
	var selected []string
	for name, isSelected := range m.selectedItems {
		if isSelected {
			selected = append(selected, name)
		}
	}
	return selected
}
