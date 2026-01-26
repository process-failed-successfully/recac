package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive command explorer",
	Long:  `Launch an interactive TUI to explore, search, and view help for all available recac commands.`,
	RunE:  runMenu,
}

func init() {
	rootCmd.AddCommand(menuCmd)
}

func runMenu(cmd *cobra.Command, args []string) error {
	commands := collectCommands(rootCmd)

	items := make([]list.Item, len(commands))
	for i, c := range commands {
		items[i] = commandItem{
			title:       c.Use,
			description: c.Short,
			cmd:         c,
		}
	}

	m := newMenuModel(items)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running menu: %w", err)
	}
	return nil
}

// Data structures

type commandItem struct {
	title       string
	description string
	cmd         *cobra.Command
}

func (i commandItem) Title() string       { return i.title }
func (i commandItem) Description() string { return i.description }
func (i commandItem) FilterValue() string { return i.title + " " + i.description }

// Model

type menuModel struct {
	list          list.Model
	viewport      viewport.Model
	showHelp      bool
	selectedCmd   *cobra.Command
	quitting      bool
	width, height int
}

func newMenuModel(items []list.Item) menuModel {
	delegate := list.NewDefaultDelegate()
	l := list.New(items, delegate, 0, 0)
	l.Title = "recac commands"
	l.SetShowHelp(false) // We use our own help view logic

	return menuModel{
		list: l,
	}
}

func (m menuModel) Init() tea.Cmd {
	return nil
}

func (m menuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetWidth(msg.Width)
		m.list.SetHeight(msg.Height)

		// Update viewport size
		headerHeight := 2 // rough estimate
		m.viewport = viewport.New(msg.Width, msg.Height-headerHeight)
		m.viewport.SetContent(m.getHelpContent())

	case tea.KeyMsg:
		if m.showHelp {
			switch msg.String() {
			case "q", "esc":
				m.showHelp = false
				return m, nil
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		} else {
			switch msg.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				if i, ok := m.list.SelectedItem().(commandItem); ok {
					m.selectedCmd = i.cmd
					m.showHelp = true
					m.viewport = viewport.New(m.width, m.height-2)
					m.viewport.SetContent(m.getHelpContent())
					// Reset viewport position
					m.viewport.GotoTop()
				}
				return m, nil
			}
		}
	}

	if !m.showHelp {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m menuModel) View() string {
	if m.quitting {
		return ""
	}

	if m.showHelp {
		titleStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#7653FF")).
			Padding(0, 1)

		title := titleStyle.Render(fmt.Sprintf(" Help: %s ", m.selectedCmd.CommandPath()))
		line := strings.Repeat("─", max(0, m.width-lipgloss.Width(title)))
		header := lipgloss.JoinHorizontal(lipgloss.Center, title, line)

		return fmt.Sprintf("%s\n%s", header, m.viewport.View())
	}

	return m.list.View()
}

func (m menuModel) getHelpContent() string {
	if m.selectedCmd == nil {
		return ""
	}

	// We want to capture the command's help output.
	// We can't easily capture stdout of Help() without jumping through hoops,
	// but we can generate it.

	// Use standard cobra help generation
	// But simply accessing cmd.Long or cmd.UsageString might be enough?
	// Let's assume Long description + Usage

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s\n\n", m.selectedCmd.Name()))
	sb.WriteString(m.selectedCmd.Short + "\n\n")

	if m.selectedCmd.Long != "" {
		sb.WriteString("## Description\n\n")
		sb.WriteString(m.selectedCmd.Long + "\n\n")
	}

	sb.WriteString("## Usage\n\n")
	sb.WriteString(m.selectedCmd.UseLine() + "\n\n")

	if m.selectedCmd.Example != "" {
		sb.WriteString("## Examples\n\n")
		sb.WriteString(m.selectedCmd.Example + "\n\n")
	}

	// Flags?
	// cobra doesn't expose a simple "FlagsAsString" easily without HelpFunc.
	// But we can try UsageString() which includes flags usually.

	// Actually, let's try to get the full help.
	// Since we are inside the same process, we can try to render it.
	// But Cobra writes to Out/Err.
	// Let's just stick to our manual rendering for now to be safe and clean.

	return sb.String()
}

// Helper functions

func collectCommands(root *cobra.Command) []*cobra.Command {
	var commands []*cobra.Command

	// Recursive walk
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if !c.IsAvailableCommand() {
			return
		}

		// We only want leaf commands or commands that do something?
		// Actually, listing groupings is also useful.
		// But let's flatten the list for fuzzy search.

		// If it has subcommands, should we list it?
		// Yes, because `recac todo` is a command group but also has help.

		// To display the full path (e.g. "todo solve")
		// we rely on c.CommandPath() or c.Use.
		// c.Use usually is just the name.
		// But c.CommandPath() gives the full path "recac todo solve".
		// We should strip "recac ".

		// Let's clone the command struct gently or just wrap it.
		// We use the pointer.

		commands = append(commands, c)

		for _, sub := range c.Commands() {
			walk(sub)
		}
	}

	for _, sub := range root.Commands() {
		walk(sub)
	}

	// Sort by command path
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].CommandPath() < commands[j].CommandPath()
	})

	return commands
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
