package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/architecture"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var visualizeCmd = &cobra.Command{
	Use:   "visualize [path]",
	Short: "Interactively explore the system architecture",
	Long:  `Launches an interactive TUI to visualize components, contracts, and data flow defined in architecture.yaml.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := ".recac/architecture/architecture.yaml"
		if len(args) > 0 {
			path = args[0]
			// If user provides a directory, append filename
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				path = filepath.Join(path, "architecture.yaml")
			}
		}

		arch, err := loadArchitecture(path)
		if err != nil {
			return fmt.Errorf("failed to load architecture from %s: %w", path, err)
		}

		p := tea.NewProgram(initialModel(arch), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running visualize TUI: %w", err)
		}
		return nil
	},
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
}

func loadArchitecture(path string) (*architecture.SystemArchitecture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return nil, err
	}
	return &arch, nil
}

// Model Definitions

type item struct {
	component architecture.Component
}

func (i item) Title() string       { return i.component.ID }
func (i item) Description() string { return i.component.Type }
func (i item) FilterValue() string { return i.component.ID }

type focus int

const (
	focusList focus = iota
	focusDetails
)

type ArchitectModel struct {
	list     list.Model
	viewport viewport.Model
	arch     *architecture.SystemArchitecture
	selected *architecture.Component
	focused  focus
	ready    bool
	width    int
	height   int
}

func initialModel(arch *architecture.SystemArchitecture) ArchitectModel {
	items := make([]list.Item, len(arch.Components))
	for i, c := range arch.Components {
		items[i] = item{component: c}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "System Components"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = lipgloss.NewStyle().MarginLeft(2)

	return ArchitectModel{
		list:    l,
		arch:    arch,
		focused: focusList,
	}
}

func (m ArchitectModel) Init() tea.Cmd {
	return nil
}

func (m ArchitectModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab":
			if m.focused == focusList {
				m.focused = focusDetails
			} else {
				m.focused = focusList
			}
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(0, 0)
			m.viewport.HighPerformanceRendering = false
			m.ready = true
		}

		listWidth := int(float64(m.width) * 0.3)
		viewWidth := m.width - listWidth - 4

		m.list.SetSize(listWidth, m.height-2)
		m.viewport.Width = viewWidth
		m.viewport.Height = m.height - 2

		// Always update list on resize to fix pagination
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		// Ensure viewport content is set/reflowed on resize
		if selectedItem := m.list.SelectedItem(); selectedItem != nil {
			if i, ok := selectedItem.(item); ok {
				m.selected = &i.component
				m.viewport.SetContent(renderDetails(m.selected, m.arch))
			}
		}

		return m, tea.Batch(cmds...)
	}

	// Update based on focus
	if m.focused == focusList {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

		// Sync viewport content only when navigating list
		if selectedItem := m.list.SelectedItem(); selectedItem != nil {
			if i, ok := selectedItem.(item); ok {
				// Only update if selection CHANGED
				if m.selected == nil || m.selected.ID != i.component.ID {
					m.selected = &i.component
					m.viewport.SetContent(renderDetails(m.selected, m.arch))
					// Reset viewport scroll to top
					m.viewport.YOffset = 0
				}
			}
		}
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ArchitectModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	borderColor := lipgloss.Color("62") // Default purple
	focusColor := lipgloss.Color("69")  // Focused blue

	listBorder := borderColor
	if m.focused == focusList {
		listBorder = focusColor
	}

	viewBorder := borderColor
	if m.focused == focusDetails {
		viewBorder = focusColor
	}

	listStyle := lipgloss.NewStyle().
		Width(int(float64(m.width) * 0.3)).
		Height(m.height - 2).
		MarginRight(2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(listBorder)

	viewStyle := lipgloss.NewStyle().
		Width(m.width - int(float64(m.width)*0.3) - 4).
		Height(m.height - 2).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(viewBorder).
		Padding(1, 2)

	listView := listStyle.Render(m.list.View())
	detailView := viewStyle.Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, detailView)
}

func renderDetails(c *architecture.Component, arch *architecture.SystemArchitecture) string {
	if c == nil {
		return "Select a component to view details."
	}

	var sb strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		Underline(true).
		MarginBottom(1)

	sb.WriteString(titleStyle.Render(c.ID))
	sb.WriteString("\n")

	// Type & Desc
	sb.WriteString(fmt.Sprintf("Type: %s\n", c.Type))
	sb.WriteString(fmt.Sprintf("\n%s\n\n", c.Description))

	// Section Helper
	sectionHeader := lipgloss.NewStyle().
		Foreground(lipgloss.Color("86")).
		Bold(true).
		MarginTop(1).
		Render

	// Consumes
	if len(c.Consumes) > 0 {
		sb.WriteString(sectionHeader("Consumes:") + "\n")
		for _, inp := range c.Consumes {
			sb.WriteString(fmt.Sprintf("• [%s] -> %s (%s)\n", inp.Source, inp.Type, inp.Schema))
		}
		sb.WriteString("\n")
	}

	// Produces
	if len(c.Produces) > 0 {
		sb.WriteString(sectionHeader("Produces:") + "\n")
		for _, out := range c.Produces {
			target := out.Target
			if target == "" {
				target = "(broadcast)"
			}
			sb.WriteString(fmt.Sprintf("• -> [%s] : %s (%s)\n", target, out.Type, out.Schema))
		}
		sb.WriteString("\n")
	}

	// Contracts
	if len(c.Contracts) > 0 {
		sb.WriteString(sectionHeader("Contracts:") + "\n")
		for _, ctr := range c.Contracts {
			sb.WriteString(fmt.Sprintf("• %s (%s)\n", ctr.Path, ctr.Type))
		}
		sb.WriteString("\n")
	}

	// Implementation Steps
	if len(c.ImplementationSteps) > 0 {
		sb.WriteString(sectionHeader("Implementation Steps:") + "\n")
		for i, step := range c.ImplementationSteps {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
		sb.WriteString("\n")
	}

	// ASCII Graph (Mini)
	sb.WriteString(sectionHeader("Dependency Graph:") + "\n")
	sb.WriteString("```mermaid\n")
	sb.WriteString(generateMiniGraph(c, arch))
	sb.WriteString("\n```\n")

	return sb.String()
}

func generateMiniGraph(c *architecture.Component, arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Identify upstream dependencies
	for _, inp := range c.Consumes {
		sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", inp.Source, inp.Type, c.ID))
	}

	// Identify downstream dependencies
	// 1. Direct targets in Produces
	for _, out := range c.Produces {
		if out.Target != "" {
			sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", c.ID, out.Type, out.Target))
		}
	}

	// 2. Implicit consumers (scan other components)
	for _, other := range arch.Components {
		if other.ID == c.ID {
			continue
		}
		for _, inp := range other.Consumes {
			if inp.Source == c.ID {
				sb.WriteString(fmt.Sprintf("    %s -->|%s| %s\n", c.ID, inp.Type, other.ID))
			}
		}
	}

	// Style the current node
	sb.WriteString(fmt.Sprintf("    style %s fill:#f9f,stroke:#333,stroke-width:2px\n", c.ID))

	// Just show the text representation for now as bubbletea doesn't render mermaid directly.
	// Users can copy-paste to mermaid live editor.
	return sb.String()
}
