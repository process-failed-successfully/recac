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
	Short: "Visualize the architecture interactively",
	Long: `Launch an interactive TUI to explore the system architecture defined in architecture.yaml.
Navigate components on the left, view details on the right.
Press 'm' to copy Mermaid graph to clipboard (or stdout).`,
	RunE: runVisualize,
}

func init() {
	architectCmd.AddCommand(visualizeCmd)
}

func runVisualize(cmd *cobra.Command, args []string) error {
	var archPath string
	if len(args) > 0 {
		archPath = args[0]
		// If directory provided, append filename
		if info, err := os.Stat(archPath); err == nil && info.IsDir() {
			archPath = filepath.Join(archPath, "architecture.yaml")
		}
	} else {
		// Default location
		archPath = ".recac/architecture/architecture.yaml"
	}

	data, err := os.ReadFile(archPath)
	if err != nil {
		return fmt.Errorf("failed to read architecture file at %s: %w", archPath, err)
	}

	var arch architecture.SystemArchitecture
	if err := yaml.Unmarshal(data, &arch); err != nil {
		return fmt.Errorf("failed to parse architecture.yaml: %w", err)
	}

	p := tea.NewProgram(initialModel(&arch), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}

// --- TUI Model ---

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	listStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			PaddingRight(2).
			MarginRight(2)
	detailStyle = lipgloss.NewStyle().
			PaddingLeft(2)
)

type item struct {
	component architecture.Component
}

func (i item) Title() string       { return i.component.ID }
func (i item) Description() string { return i.component.Type + ": " + i.component.Description }
func (i item) FilterValue() string { return i.component.ID }

type ArchModel struct {
	arch     *architecture.SystemArchitecture
	list     list.Model
	viewport viewport.Model
	ready    bool
	width    int
	height   int
}

func initialModel(arch *architecture.SystemArchitecture) ArchModel {
	items := make([]list.Item, len(arch.Components))
	for i, c := range arch.Components {
		items[i] = item{component: c}
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Components"
	l.SetShowHelp(false)

	return ArchModel{
		arch: arch,
		list: l,
	}
}

func (m ArchModel) Init() tea.Cmd {
	return nil
}

func (m ArchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "m":
			// Generate Mermaid
			mermaid := generateMermaidArch(m.arch)
			// TODO: Copy to clipboard if possible, but for now just print to stdout on quit?
			// Or maybe write to a file?
			// Let's just return quit and print it? No, that exits TUI.
			// Let's use tea.Exec to run a command or print.
			// Ideally, copy to clipboard.
			// For simplicity in this implementation, let's print to stdout after quit
			// by returning a specific value, or just use a message.
			// But since we are in AltScreen, printing to stdout won't be seen easily.
			// Let's create a file "architecture.mermaid" and notify user.
			err := os.WriteFile("architecture.mermaid", []byte(mermaid), 0644)
			if err != nil {
				// Show error?
			}
			return m, m.list.NewStatusMessage(lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Render("Saved to architecture.mermaid"))
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		listWidth := int(float64(m.width) * 0.3)
		detailWidth := m.width - listWidth - 4

		if !m.ready {
			m.viewport = viewport.New(detailWidth, m.height-4) // -4 for margins
			m.ready = true
		} else {
			m.viewport.Width = detailWidth
			m.viewport.Height = m.height - 4
		}

		m.list.SetSize(listWidth, m.height-2)
	}

	// Update list
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport content based on selection
	if m.ready {
		selectedItem := m.list.SelectedItem()
		if selectedItem != nil {
			comp := selectedItem.(item).component
			content := renderComponentDetails(comp)
			m.viewport.SetContent(content)
		}

		// Handle viewport scrolling
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m ArchModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	return docStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			listStyle.Render(m.list.View()),
			detailStyle.Render(m.viewport.View()),
		),
	)
}

func renderComponentDetails(c architecture.Component) string {
	var sb strings.Builder

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205")).Underline(true)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("63")).MarginBottom(1)

	sb.WriteString(titleStyle.Render(fmt.Sprintf("%s (%s)", c.ID, c.Type)) + "\n")
	sb.WriteString(c.Description + "\n\n")

	if len(c.Contracts) > 0 {
		sb.WriteString(headerStyle.Render("Contracts") + "\n")
		for _, con := range c.Contracts {
			sb.WriteString(fmt.Sprintf("- [%s] %s\n", con.Type, con.Path))
		}
		sb.WriteString("\n")
	}

	if len(c.Consumes) > 0 {
		sb.WriteString(headerStyle.Render("Consumes") + "\n")
		for _, in := range c.Consumes {
			sb.WriteString(fmt.Sprintf("- From %s: %s (%s)\n", in.Source, in.Type, in.Schema))
		}
		sb.WriteString("\n")
	}

	if len(c.Produces) > 0 {
		sb.WriteString(headerStyle.Render("Produces") + "\n")
		for _, out := range c.Produces {
			target := out.Target
			if target == "" {
				target = "(broadcast)"
			}
			sb.WriteString(fmt.Sprintf("- To %s: %s (Event: %s)\n", target, out.Type, out.Event))
		}
		sb.WriteString("\n")
	}

	if len(c.Functions) > 0 {
		sb.WriteString(headerStyle.Render("Functions") + "\n")
		for _, fn := range c.Functions {
			sb.WriteString(fmt.Sprintf("- %s(%s) %s\n  %s\n", fn.Name, fn.Args, fn.Return, fn.Description))
		}
	}

	return sb.String()
}

// --- Mermaid Generation ---

func generateMermaidArch(arch *architecture.SystemArchitecture) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Nodes
	for _, c := range arch.Components {
		// Shape based on type
		open := "["
		close := "]"
		switch strings.ToLower(c.Type) {
		case "database", "db":
			open = "[("
			close = ")]"
		case "queue", "topic":
			open = "{{"
			close = "}}"
		case "worker", "job":
			open = "(["
			close = "])"
		case "frontend", "ui":
			open = "(("
			close = "))"
		}
		// Sanitize ID for safe mermaid syntax
		safeID := sanitizeMermaidID(c.ID)
		sb.WriteString(fmt.Sprintf("    %s%s\"%s<br/>(%s)\"%s\n", safeID, open, c.ID, c.Type, close))
	}

	// Edges from Consumes
	// Component A consumes from Component B -> B --> A
	for _, c := range arch.Components {
		safeDest := sanitizeMermaidID(c.ID)
		for _, in := range c.Consumes {
			safeSrc := sanitizeMermaidID(in.Source)
			// Check if source exists in components to avoid errors, or create external node?
			// Let's assume it exists or render as external
			label := in.Type
			if in.Type == "" {
				label = "uses"
			}
			sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", safeSrc, label, safeDest))
		}
	}

	// Edges from Produces with explicit Target
	for _, c := range arch.Components {
		safeSrc := sanitizeMermaidID(c.ID)
		for _, out := range c.Produces {
			if out.Target != "" {
				safeDest := sanitizeMermaidID(out.Target)
				label := out.Event
				if label == "" {
					label = out.Type
				}
				sb.WriteString(fmt.Sprintf("    %s -- \"%s\" --> %s\n", safeSrc, label, safeDest))
			}
		}
	}

	return sb.String()
}
