package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take a guided tour of RECAC features",
	Long:  "Launch an interactive TUI to explore the features, architecture, and workflow of RECAC.",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(initialTourModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("tour failed: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

type tourModel struct {
	slides   []string
	current  int
	renderer *glamour.TermRenderer
	width    int
	height   int
	quitting bool
}

func initialTourModel() tourModel {
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return tourModel{
		slides:   tourSlides,
		current:  0,
		renderer: r,
	}
}

func (m tourModel) Init() tea.Cmd {
	return nil
}

func (m tourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		case "right", "l", "enter", " ", "space":
			if m.current < len(m.slides)-1 {
				m.current++
			}
		case "left", "h", "backspace":
			if m.current > 0 {
				m.current--
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.renderer != nil {
			m.renderer, _ = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.width-4), // slightly less than full width
			)
		}
	}
	return m, nil
}

func (m tourModel) View() string {
	if m.quitting {
		return "Thanks for taking the tour! Bye!\n"
	}

	if m.renderer == nil {
		// Fallback if renderer failed to init (e.g. in tests or weird envs)
		return m.slides[m.current]
	}

	content, err := m.renderer.Render(m.slides[m.current])
	if err != nil {
		return fmt.Sprintf("Error rendering slide: %v", err)
	}

	// Add footer
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Align(lipgloss.Center).
		Width(m.width)

	footerText := fmt.Sprintf("Slide %d/%d • ← Prev • Next → • q Quit", m.current+1, len(m.slides))
	footer := footerStyle.Render(footerText)

	// Combine content and footer
	// We might need to handle height, but for now simple concatenation
	return fmt.Sprintf("%s\n\n%s", content, footer)
}

var tourSlides = []string{
	`
# Welcome to RECAC 🚀

**Rewrite of Combined Autonomous Coding**

RECAC is a next-generation autonomous coding framework designed for:
- 🏗️ **Scale**: Distributed architecture with Orchestrator and Agents.
- 🤖 **Intelligence**: Multi-agent support (OpenAI, Gemini, Ollama).
- 🔄 **Automation**: Full lifecycle management from Jira ticket to PR.

Press **Space** or **Right Arrow** to continue.
`,
	`
# 1. Distributed Architecture 🌐

Gone is the single monolithic binary. RECAC now operates as:

1.  **Orchestrator**: The brain. It polls for work (Jira, Files) and spawns agents.
2.  **Agents**: The workers. Ephemeral processes (Docker containers or K8s Jobs) that do the actual coding.

This allows you to run **hundreds** of agents in parallel on a Kubernetes cluster!
`,
	`
# 2. The Orchestrator 🎼

The orchestrator is your command center.

Run it locally:
` + "```bash" + `
recac orchestrate --mode local --jira-label "recac-agent"
` + "```" + `

Or deploy to Kubernetes:
` + "```bash" + `
helm install recac ./deploy/helm/recac
` + "```" + `

It automatically handles:
- 📥 Polling Jira for new tickets.
- 🐳 Spawning Docker containers for each task.
- 📊 Aggregating logs and status.
`,
	`
# 3. The Agent 🕵️

The agent is where the magic happens. Each agent:

1.  **Clones** your repository.
2.  **Reads** the Jira ticket instructions.
3.  **Plans** the changes using an LLM.
4.  **Implements** the code.
5.  **Verifies** it with tests.
6.  **Submits** a Pull Request.

You can also run an agent manually for debugging:
` + "```bash" + `
recac-agent --jira RD-123 --repo-url ...
` + "```" + `
`,
	`
# 4. Monitoring 📊

Keep track of your army of agents with the TUI Dashboard.

` + "```bash" + `
recac ui
` + "```" + `

(Note: The dashboard is currently being integrated into the ` + "`orchestrate`" + ` command, but you can view logs via ` + "`recac logs`" + ` or check the ` + "`recac job`" + ` command suite).

And of course, **Jira** is updated in real-time with comments and status transitions.
`,
	`
# 5. Gamification 🎮

Coding should be fun! RECAC tracks your contributions and awards XP.

Check your leaderboard:
` + "```bash" + `
recac gamify
` + "```" + `

Earn badges for:
- 🐛 Fixing bugs
- 📚 Writing documentation
- 🧪 Adding tests

Get to the top of the leaderboard!
`,
	`
# Ready to start? 🏁

1.  **Configure**: Edit ` + "`~/.recac.yaml`" + ` with your API keys.
2.  **Create**: Make a Jira ticket with label ` + "`recac-agent`" + `.
3.  **Launch**: Run ` + "`recac orchestrate`" + `.

Happy Coding! 🎉

Press **q** to quit.
`,
}
