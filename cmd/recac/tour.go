package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Interactive tour of RECAC features",
	Long:  `Launch an interactive TUI tour to learn about RECAC's features, architecture, and usage.`,
	RunE:  runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

func runTour(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(initialTourModel())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tour failed: %w", err)
	}
	return nil
}

type tourModel struct {
	slides   []string
	current  int
	width    int
	height   int
	keys     keyMap
	help     help.Model
	renderer *glamour.TermRenderer
}

type keyMap struct {
	Next key.Binding
	Prev key.Binding
	Quit key.Binding
}

// ShortHelp returns keybindings to be shown in the mini help view.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Next, k.Prev, k.Quit}
}

// FullHelp returns keybindings for the expanded help view.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{k.ShortHelp()}
}

var keys = keyMap{
	Next: key.NewBinding(
		key.WithKeys("right", "l", "n", "space", "enter"),
		key.WithHelp("→/space", "next"),
	),
	Prev: key.NewBinding(
		key.WithKeys("left", "h", "p", "backspace"),
		key.WithHelp("←/backspace", "prev"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q/esc", "quit"),
	),
}

func initialTourModel() tourModel {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		fmt.Printf("Warning: could not initialize glamour renderer: %v\n", err)
	}

	return tourModel{
		slides:   generateSlides(),
		current:  0,
		keys:     keys,
		help:     help.New(),
		renderer: r,
	}
}

func (m tourModel) Init() tea.Cmd {
	return nil
}

func (m tourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Next):
			if m.current < len(m.slides)-1 {
				m.current++
			}
		case key.Matches(msg, m.keys.Prev):
			if m.current > 0 {
				m.current--
			}
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
	}
	return m, nil
}

func (m tourModel) View() string {
	if m.renderer == nil {
		return "Error: Renderer not initialized"
	}

	content := m.slides[m.current]
	rendered, err := m.renderer.Render(content)
	if err != nil {
		return fmt.Sprintf("Error rendering slide: %v", err)
	}

	helpView := m.help.View(m.keys)

	// Join vertically
	return lipgloss.JoinVertical(lipgloss.Left, rendered, "\n", helpView)
}

func generateSlides() []string {
	return []string{
		`
# Welcome to RECAC 🚀

**Rewrite of Combined Autonomous Coding**

RECAC is a comprehensive framework for autonomous coding, designed for scale and reliability.

It helps you:
- 🤖 Automate coding tasks with AI agents.
- 🏗️ Orchestrate distributed workloads.
- 📊 Track progress with TUI and integrations.

*Press Next (→) to continue...*
`,
		`
# Core Components 🧩

RECAC is split into specialized binaries:

1. **Orchestrator** 🎼
   - Polls tasks (Jira, Files).
   - Spawns agents (Docker, K8s).
   - Manages lifecycle.

2. **RECAC Agent** 🕵️
   - The core intelligence.
   - Clones repo, analyzes, codes, verifies.
   - Runs in isolation.

`,
		`
# Distributed Architecture 🌐

Designed for Kubernetes and Docker.

- **Scale Independently**: Run 1 or 100 agents.
- **Poll-Spawn-Verify**:
  1. Orchestrator polls Jira.
  2. Spawns an Agent Job.
  3. Agent pushes code & notifies.

No more single monolithic process blocking your terminal!
`,
		`
# Architect Mode 🏛️

Generate systems from high-level specs.

1. Write ` + "`app_spec.txt`" + `.
2. Run ` + "`recac architect --spec app_spec.txt`" + `.
3. Get:
   - ` + "`architecture.yaml`" + `
   - ` + "`contracts/*.yaml`" + `
   - **Jira Tickets** (automatically generated!)

From idea to implementation plan in minutes.
`,
		`
# Getting Started 🚦

**Local Mode**:
` + "```bash" + `
# Run Orchestrator locally
./bin/orchestrator --mode local --jira-label "recac-agent"
` + "```" + `

**Kubernetes**:
` + "```bash" + `
helm install recac ./deploy/helm/recac
` + "```" + `

Check ` + "`README.md`" + ` for full details.
`,
		`
# Thank You! 🎉

Explore more commands:
- ` + "`recac check`" + `: Verify code quality.
- ` + "`recac explain`" + `: Explain code with AI.
- ` + "`recac presentation`" + `: Generate slides from git history.

Documentation: [GitHub Repository](https://github.com/process-failed-successfully/recac)

*Press 'q' to quit.*
`,
	}
}
