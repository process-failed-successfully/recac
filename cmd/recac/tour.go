package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Interactive tour of the recac codebase",
	Long:  `Launch an interactive TUI tour that guides you through the key components of the recac project structure.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		items := []list.Item{
			TourStop{
				TitleText: "Welcome to recac",
				Desc:      "Introduction to the autonomous coding framework.",
				Content: `
# Welcome to recac

**recac** (Rewrite of Combined Autonomous Coding) is a distributed framework designed for autonomous coding agents. It orchestrates AI agents to solve tasks, from Jira tickets to complex architectural changes.

This tour will guide you through the key components of the system.

Use the **Arrow Keys** to navigate and **Esc** or **q** to quit.
`,
			},
			TourStop{
				TitleText: "Orchestrator",
				Desc:      "cmd/orchestrator: The brain of the operation.",
				Content: `
# Orchestrator (cmd/orchestrator)

The **Orchestrator** is responsible for managing the lifecycle of coding sessions. It:

- Polls Jira for new tickets.
- Spawns agent containers or Kubernetes jobs.
- Monitors agent health and progress.

Key file: ` + "`cmd/orchestrator/main.go`" + `
`,
			},
			TourStop{
				TitleText: "Agent",
				Desc:      "cmd/agent: The autonomous worker.",
				Content: `
# Agent (cmd/agent)

The **Agent** is the worker process that executes coding tasks. It runs inside a Docker container and:

- Clones the repository.
- Analyzes the codebase.
- Plans and implements changes.
- Runs tests and verifies correctness.

Key file: ` + "`cmd/agent/main.go`" + `
`,
			},
			TourStop{
				TitleText: "Runner Logic",
				Desc:      "internal/runner: Core workflow loop.",
				Content: `
# Runner Logic (internal/runner)

The **Runner** package contains the core logic for the agent's execution loop. It handles:

- The *Poll-Spawn-Verify* cycle.
- State management for the agent session.
- Interactions with the Docker daemon.

Key file: ` + "`internal/runner/session.go`" + `
`,
			},
			TourStop{
				TitleText: "Analysis Tools",
				Desc:      "internal/analysis: Code metrics and insights.",
				Content: `
# Analysis Tools (internal/analysis)

Recac includes a suite of static analysis tools to help agents understand code quality. These tools calculate:

- **Cyclomatic Complexity**: ` + "`internal/analysis/complexity.go`" + `
- **Dead Code**: ` + "`internal/analysis/deadcode.go`" + `
- **Hotspots**: Identify frequently changed files.

These metrics guide the agent's refactoring decisions.
`,
			},
			TourStop{
				TitleText: "TUI Components",
				Desc:      "internal/tui: User Interface library.",
				Content: `
# TUI Components (internal/tui)

The **TUI** package provides reusable Bubble Tea components for the CLI. It includes:

- **Dashboard**: A real-time view of orchestrator status.
- **Spinners & Progress Bars**: Visual feedback for long-running tasks.

Key file: ` + "`internal/tui/dashboard.go`" + `
`,
			},
		}

		m := NewTourModel(items)
		p := tea.NewProgram(m, tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running tour: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

// TourStop represents a step in the tour.
type TourStop struct {
	TitleText string
	Desc      string
	Content   string
}

func (i TourStop) Title() string       { return i.TitleText }
func (i TourStop) Description() string { return i.Desc }
func (i TourStop) FilterValue() string { return i.TitleText }

type tourModel struct {
	list     list.Model
	viewport viewport.Model
	ready    bool
	renderer *glamour.TermRenderer
}

func NewTourModel(items []list.Item) tourModel {
	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Recac Codebase Tour"
	l.SetShowHelp(false)

	// Initialize Glamour renderer
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return tourModel{
		list:     l,
		renderer: renderer,
	}
}

func (m tourModel) Init() tea.Cmd {
	return nil
}

func (m tourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = false
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		// Resize list
		// We want the list to take up 30% of the width
		listWidth := int(float64(msg.Width) * 0.3)
		if listWidth < 20 {
			listWidth = 20
		}
		m.list.SetSize(listWidth, msg.Height-verticalMarginHeight)

		// Set viewport width to remainder
		m.viewport.Width = msg.Width - listWidth - 2 // -2 for margins/borders if any
	}

	// Update list
	var listModel list.Model
	listModel, cmd = m.list.Update(msg)
	m.list = listModel
	cmds = append(cmds, cmd)

	// Update viewport content based on selection
	if m.ready {
		selectedItem := m.list.SelectedItem()
		if selectedItem != nil {
			stop, ok := selectedItem.(TourStop)
			if ok {
				var rendered string
				var err error
				if m.renderer != nil {
					rendered, err = m.renderer.Render(stop.Content)
				} else {
					err = fmt.Errorf("renderer not initialized")
				}

				if err != nil {
					rendered = stop.Content // Fallback
				}
				m.viewport.SetContent(rendered)
			}
		}
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m tourModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Render List
	listView := m.list.View()

	// Render Viewport
	viewportView := m.viewport.View()

	// Style split pane
	return lipgloss.JoinHorizontal(lipgloss.Top, listView, viewportView)
}

func (m tourModel) headerView() string {
	return ""
}

func (m tourModel) footerView() string {
	return ""
}
