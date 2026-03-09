package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	depsDocStyle = lipgloss.NewStyle().Margin(1, 2)

	depsListStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("240")).
			MarginRight(2)

	depsDetailStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	depsTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			MarginBottom(1)

	depsMetricLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240")).
				Width(12)

	depsMetricValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true)
)

// DependencyGraph represents the dependency structure
type DependencyGraph struct {
	Outgoing map[string][]string // Package -> Dependencies (Ce)
	Incoming map[string][]string // Package -> Importers (Ca)
}

// PackageMetric holds the calculated metrics for a package
type PackageMetric struct {
	Name        string
	Afferent    int     // Ca: Incoming dependencies (stability)
	Efferent    int     // Ce: Outgoing dependencies (responsibility)
	Instability float64 // I = Ce / (Ca + Ce). 0 = Stable, 1 = Unstable
}

// item implements list.Item
type depsItem struct {
	metric PackageMetric
}

func (i depsItem) Title() string { return i.metric.Name }
func (i depsItem) Description() string {
	return fmt.Sprintf("I: %.2f | Ca: %d | Ce: %d", i.metric.Instability, i.metric.Afferent, i.metric.Efferent)
}
func (i depsItem) FilterValue() string { return i.metric.Name }

type DepsModel struct {
	graph       DependencyGraph
	metrics     map[string]PackageMetric
	list        list.Model
	viewport    viewport.Model
	ready       bool
	width       int
	height      int
	selectedPkg string
}

func NewDepsModel(outgoing map[string][]string) DepsModel {
	// 1. Build Graph & Calculate Metrics
	graph := DependencyGraph{
		Outgoing: outgoing,
		Incoming: make(map[string][]string),
	}
	metrics := make(map[string]PackageMetric)

	// Collect all unique packages
	allPkgs := make(map[string]bool)
	for pkg, deps := range outgoing {
		allPkgs[pkg] = true
		for _, dep := range deps {
			allPkgs[dep] = true
			graph.Incoming[dep] = append(graph.Incoming[dep], pkg)
		}
	}

	var items []list.Item
	for pkg := range allPkgs {
		ce := len(graph.Outgoing[pkg])
		ca := len(graph.Incoming[pkg])
		instability := 0.0
		if ce+ca > 0 {
			instability = float64(ce) / float64(ce+ca)
		}

		m := PackageMetric{
			Name:        pkg,
			Afferent:    ca,
			Efferent:    ce,
			Instability: instability,
		}
		metrics[pkg] = m
		items = append(items, depsItem{metric: m})
	}

	// Sort items by instability (descending) by default, or name?
	// Let's sort by Name for easy finding
	sort.Slice(items, func(i, j int) bool {
		return items[i].(depsItem).Title() < items[j].(depsItem).Title()
	})

	// 2. Initialize List
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Package Dependencies"
	l.SetShowHelp(false)

	return DepsModel{
		graph:   graph,
		metrics: metrics,
		list:    l,
	}
}

func (m DepsModel) Init() tea.Cmd {
	return nil
}

func (m DepsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout: 40% list, 60% detail
		listWidth := int(float64(m.width) * 0.4)
		detailWidth := m.width - listWidth - 4

		m.list.SetWidth(listWidth)
		m.list.SetHeight(m.height - 4)

		if !m.ready {
			m.viewport = viewport.New(detailWidth, m.height-4)
			m.ready = true
		} else {
			m.viewport.Width = detailWidth
			m.viewport.Height = m.height - 4
		}
	}

	newList, newCmd := m.list.Update(msg)
	m.list = newList
	cmds = append(cmds, newCmd)

	// Update selection
	if i, ok := m.list.SelectedItem().(depsItem); ok {
		if m.selectedPkg != i.metric.Name {
			m.selectedPkg = i.metric.Name
			m.updateViewport()
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *DepsModel) updateViewport() {
	if m.selectedPkg == "" {
		m.viewport.SetContent("Select a package to view details.")
		return
	}

	pkg := m.selectedPkg
	metric, ok := m.metrics[pkg]
	if !ok {
		m.viewport.SetContent("Package metrics not found.")
		return
	}

	var sb strings.Builder

	// Title
	sb.WriteString(depsTitleStyle.Render(pkg) + "\n\n")

	// Metrics
	sb.WriteString(renderMetric("Instability", fmt.Sprintf("%.2f", metric.Instability)))
	sb.WriteString(renderInstabilityBar(metric.Instability, m.viewport.Width-20) + "\n\n")

	sb.WriteString(renderMetric("Abstractness", "N/A (needs parser)")) // Placeholder
	sb.WriteString("\n")
	sb.WriteString(renderMetric("Incoming (Ca)", fmt.Sprintf("%d", metric.Afferent)))
	sb.WriteString("\n")
	sb.WriteString(renderMetric("Outgoing (Ce)", fmt.Sprintf("%d", metric.Efferent)))
	sb.WriteString("\n\n")

	// Lists
	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Imported Packages (Direct Dependencies):") + "\n")
	if len(m.graph.Outgoing[pkg]) == 0 {
		sb.WriteString("  (None)\n")
	} else {
		for _, dep := range m.graph.Outgoing[pkg] {
			sb.WriteString(fmt.Sprintf("  • %s\n", dep))
		}
	}
	sb.WriteString("\n")

	sb.WriteString(lipgloss.NewStyle().Bold(true).Render("Imported By (Reverse Dependencies):") + "\n")
	if len(m.graph.Incoming[pkg]) == 0 {
		sb.WriteString("  (None)\n")
	} else {
		for _, imp := range m.graph.Incoming[pkg] {
			sb.WriteString(fmt.Sprintf("  • %s\n", imp))
		}
	}

	m.viewport.SetContent(sb.String())
}

func renderMetric(label, value string) string {
	return depsMetricLabelStyle.Render(label+":") + " " + depsMetricValueStyle.Render(value)
}

func renderInstabilityBar(value float64, width int) string {
	if width < 10 {
		width = 10
	}
	filled := int(float64(width) * value)
	empty := width - filled

	// Color gradient: Green (Stable) -> Red (Unstable)
	color := lipgloss.Color("46") // Green
	if value > 0.7 {
		color = lipgloss.Color("196") // Red
	} else if value > 0.3 {
		color = lipgloss.Color("226") // Yellow
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return lipgloss.NewStyle().Foreground(color).Render(bar)
}

func (m DepsModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	return depsDocStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			depsListStyle.Render(m.list.View()),
			depsDetailStyle.Render(m.viewport.View()),
		),
	)
}

// StartDeps launches the interactive dependency explorer
func StartDeps(outgoing map[string][]string) error {
	m := NewDepsModel(outgoing)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
