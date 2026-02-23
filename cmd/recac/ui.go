package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)
	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

// item implements list.Item
type item struct {
	title, desc string
	cmd         *cobra.Command
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title + " " + i.desc }

// getCommands returns a list of items for the given command's subcommands.
func getCommands(cmd *cobra.Command) []list.Item {
	var items []list.Item
	for _, c := range cmd.Commands() {
		if c.Hidden || c.Deprecated != "" {
			continue
		}
		items = append(items, item{
			title: c.Name(),
			desc:  c.Short,
			cmd:   c,
		})
	}
	// Sort by title
	sort.Slice(items, func(i, j int) bool {
		return items[i].(item).Title() < items[j].(item).Title()
	})
	return items
}

type uiModel struct {
	list          list.Model
	currentCmd    *cobra.Command
	history       []*cobra.Command
	quitting      bool
	width, height int
}

func newUIModel() uiModel {
	items := getCommands(rootCmd)

	delegate := list.NewDefaultDelegate()

	l := list.New(items, delegate, 0, 0)
	l.Title = "Recac Commands"
	l.Styles.Title = titleStyle
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "go back")),
		}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("backspace"), key.WithHelp("backspace", "go back")),
		}
	}
	// Set status message lifetime to something visible
	l.StatusMessageLifetime = 5 * time.Second

	return uiModel{
		list:       l,
		currentCmd: rootCmd,
		history:    []*cobra.Command{},
	}
}

func (m uiModel) Init() tea.Cmd {
	return nil
}

func (m uiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit

		case "enter":
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				return m, nil
			}
			i, ok := selectedItem.(item)
			if ok {
				selectedCmd := i.cmd
				subItems := getCommands(selectedCmd)

				if len(subItems) > 0 {
					// Drill down
					m.history = append(m.history, m.currentCmd)
					m.currentCmd = selectedCmd
					m.list.SetItems(subItems)
					m.list.Title = "Recac > " + selectedCmd.CommandPath()
					m.list.ResetSelected()
				} else {
					// Leaf command: Copy to clipboard
					cmdStr := selectedCmd.CommandPath()
					// Try to copy
					err := clipboard.WriteAll(cmdStr)
					if err != nil {
						m.list.NewStatusMessage(statusMessageStyle("Error copying to clipboard: " + err.Error()))
					} else {
						m.list.NewStatusMessage(statusMessageStyle("Copied '" + cmdStr + "' to clipboard!"))
					}
				}
			}
			return m, nil

		case "backspace":
			if len(m.history) > 0 {
				// Go up
				prev := m.history[len(m.history)-1]
				m.history = m.history[:len(m.history)-1]
				m.currentCmd = prev

				items := getCommands(prev)
				m.list.SetItems(items)

				if prev == rootCmd {
					m.list.Title = "Recac Commands"
				} else {
					m.list.Title = "Recac > " + prev.CommandPath()
				}
				m.list.ResetSelected()
			}
			return m, nil
		}
	}

	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m uiModel) View() string {
	if m.quitting {
		return ""
	}
	return appStyle.Render(m.list.View())
}

var uiCmd = &cobra.Command{
	Use:   "ui",
	Short: "Interactive TUI for browsing and running commands",
	Long:  `An interactive terminal user interface to explore and execute recac commands.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(newUIModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("failed to run TUI: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(uiCmd)
}
