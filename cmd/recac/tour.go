package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	tourTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(0, 1)

	tourStatusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Padding(0, 1)
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Interactive tour of RECAC",
	Long:  `Launch an interactive TUI tour that explains the core concepts, architecture, and commands of the RECAC ecosystem.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(newTourModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

type TourStep struct {
	Title   string
	Content string
}

type TourModel struct {
	steps    []TourStep
	current  int
	viewport viewport.Model
	width    int
	height   int
	ready    bool
	renderer *glamour.TermRenderer
}

func newTourModel() TourModel {
	// Helper for backticks in raw strings
	bt := "`"

	steps := []TourStep{
		{
			Title: "Welcome to RECAC",
			Content: `
# Welcome to RECAC! 🚀

**Rewrite of Combined Autonomous Coding**

Recac is a powerful framework for autonomous coding agents. It helps you orchestrate, manage, and scale AI agents to solve complex coding tasks.

Use this tour to learn about:
- 🏗️  **Architecture**: How the pieces fit together.
- 🛠️  **Commands**: Essential CLI tools.
- 🔄 **Workflow**: From Jira to PR.

*Press **Enter** or **Right Arrow** to continue.*
`,
		},
		{
			Title: "Architecture Overview",
			Content: `
# 🏗️ Architecture

Recac is distributed by design:

1.  **Orchestrator**: The brain. It polls Jira/Files for tasks and spawns agents.
2.  **Agent**: The worker. It runs in Docker/K8s, clones code, and implements features.
3.  **CLI (recac)**: Your interface. Manage everything from here.

` + bt + bt + bt + `mermaid
graph LR
    J[Jira] --> O[Orchestrator]
    O --> A1[Agent 1]
    O --> A2[Agent 2]
    A1 --> G[GitHub]
` + bt + bt + bt + `

*Press **Enter** for next step.*
`,
		},
		{
			Title: "Key Commands",
			Content: `
# 🛠️ Key Commands

- **recac setup**: Configure API keys and integrations.
- **recac doctor**: Diagnose system health (Docker, Go, etc.).
- **recac agent**: Run an agent manually (good for debugging).
- **recac orchestrator**: Start the task manager.

> **Tip:** Use ` + bt + `recac help` + bt + ` to see all 100+ commands!

*Press **Enter** for next step.*
`,
		},
		{
			Title: "Workflow",
			Content: `
# 🔄 The Workflow

1.  **Create Ticket**: Create a Jira ticket with label ` + bt + `recac-agent` + bt + `.
2.  **Orchestrate**: Run ` + bt + `bin/orchestrator` + bt + `. It picks up the ticket.
3.  **Spawn**: A Docker container starts with the agent.
4.  **Implement**: The agent reads the ticket, plans, codes, and tests.
5.  **Review**: A PR is created for you to review.

*Press **Enter** for next step.*
`,
		},
		{
			Title: "Next Steps",
			Content: `
# 🚀 Ready to Launch?

1.  Run **` + bt + `recac setup` + bt + `** to configure your environment.
2.  Run **` + bt + `recac doctor` + bt + `** to ensure everything is installed.
3.  Try **` + bt + `recac agent --help` + bt + `** to explore agent options.

Enjoy coding with your autonomous team!

*Press **q** to quit.*
`,
		},
	}

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return TourModel{
		steps:    steps,
		current:  0,
		renderer: r,
	}
}

func (m TourModel) Init() tea.Cmd {
	return nil
}

func (m TourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "enter", "space", "right", "l":
			if m.current < len(m.steps)-1 {
				m.current++
				m.renderContent()
			}
		case "backspace", "left", "h":
			if m.current > 0 {
				m.current--
				m.renderContent()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
			m.renderContent()
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
			m.renderContent() // Re-render with new width
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *TourModel) renderContent() {
	if !m.ready {
		return
	}

	step := m.steps[m.current]

	// Safety check for renderer
	if m.renderer == nil {
		m.viewport.SetContent(step.Content)
		return
	}

	content, err := m.renderer.Render(step.Content)
	if err != nil {
		content = "Error rendering content: " + err.Error() + "\n\n" + step.Content
	}

	m.viewport.SetContent(content)
}

func (m TourModel) View() string {
	if !m.ready {
		return "\n  Initializing Tour..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m TourModel) headerView() string {
	title := tourTitleStyle.Render(fmt.Sprintf("Recac Tour (%d/%d): %s", m.current+1, len(m.steps), m.steps[m.current].Title))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m TourModel) footerView() string {
	info := tourStatusStyle.Render("Next: Enter/→  |  Prev: Backspace/←  |  Quit: q")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
