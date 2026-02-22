package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	titleStyle        = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(2)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("#EE6FF8"))
	detailStyle       = lipgloss.NewStyle().Padding(1, 2).Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("#874BFD"))
)

type TourStep struct {
	Title       string `json:"title"`
	File        string `json:"file"`
	Description string `json:"description"`
}

type TourModel struct {
	steps    []TourStep
	cursor   int
	focus    int // 0: List, 1: Detail
	viewport viewport.Model
	width    int
	height   int
	ready    bool
	quitting bool
	err      error
}

func (m *TourModel) Init() tea.Cmd {
	return nil
}

func (m *TourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			m.quitting = true
			return m, tea.Quit
		}

		if k := msg.String(); k == "tab" {
			m.focus = (m.focus + 1) % 2
			return m, nil
		}

		if m.focus == 0 {
			// List navigation
			if k := msg.String(); k == "up" || k == "k" {
				if m.cursor > 0 {
					m.cursor--
					m.updateViewportContent()
				}
			}

			if k := msg.String(); k == "down" || k == "j" {
				if m.cursor < len(m.steps)-1 {
					m.cursor++
					m.updateViewportContent()
				}
			}
		} else {
			// Viewport navigation
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width/2, msg.Height-5)
			m.viewport.YPosition = 0
			m.ready = true
			m.updateViewportContent()
		} else {
			m.viewport.Width = msg.Width / 2
			m.viewport.Height = msg.Height - 5
			m.updateViewportContent() // Re-render content with new width
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *TourModel) updateViewportContent() {
	if len(m.steps) == 0 {
		return
	}
	step := m.steps[m.cursor]

	content := fmt.Sprintf("# %s\n\n%s\n\n", step.Title, step.Description)

	if step.File != "" {
		fileContent, err := os.ReadFile(step.File)
		if err == nil {
			content += fmt.Sprintf("## File: %s\n\n```\n%s\n```", step.File, string(fileContent))
		} else {
			content += fmt.Sprintf("## File: %s (Could not read: %v)\n", step.File, err)
		}
	}

	m.viewport.SetContent(content)
}

func (m *TourModel) View() string {
	if m.quitting {
		return "Thanks for touring with RECAC!\n"
	}
	if !m.ready {
		return "\n  Initializing..."
	}

	// Render list
	var listBuilder strings.Builder
	listBuilder.WriteString(titleStyle.Render("Tour Itinerary") + "\n\n")

	for i, step := range m.steps {
		cursor := " "
		style := itemStyle
		if m.cursor == i {
			cursor = ">"
			style = selectedItemStyle
		}
		listBuilder.WriteString(style.Render(fmt.Sprintf("%s %s", cursor, step.Title)) + "\n")
	}

	listView := listBuilder.String()
	if m.focus == 0 {
		listView = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("205")).Render(listView)
	} else {
		listView = lipgloss.NewStyle().Padding(1).Render(listView)
	}

	// Ensure viewport content is set
	if m.viewport.View() == "" {
		m.updateViewportContent()
	}

	var currentDetailStyle lipgloss.Style
	if m.focus == 1 {
		currentDetailStyle = detailStyle.BorderForeground(lipgloss.Color("205")) // Highlight
	} else {
		currentDetailStyle = detailStyle
	}
	detailView := currentDetailStyle.Render(m.viewport.View())

	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/3).Render(listView),
		lipgloss.NewStyle().Width(m.width*2/3).Render(detailView),
	)
}

var tourCmd = &cobra.Command{
	Use:   "tour [path]",
	Short: "Interactive guided tour of the codebase",
	Long: `Analyze the project structure and generate an interactive guided tour using AI.
This helps new developers understand the architecture and key components quickly.`,
	RunE: runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

func runTour(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Generate Context
	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing codebase structure...")

	opts := ContextOptions{
		Roots:     []string{root},
		MaxSize:   1024 * 1024, // 1MB
		Tree:      true,
		NoContent: true,
	}

	var readmeContent string
	readmePath := filepath.Join(root, "README.md")
	if b, err := os.ReadFile(readmePath); err == nil {
		readmeContent = string(b)
	}

	treeContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate context: %w", err)
	}

	// 2. Prompt Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a senior software architect onboarding a new developer.
Create a guided tour itinerary for this project based on the file structure and README.

README:
%s

File Tree:
%s

Generate a JSON list of 5-10 key steps. Each step should focus on a specific file or concept.
The steps should follow a logical order (e.g., Entry point -> Core Logic -> Utils -> Tests).

Return JSON format ONLY:
[
  {
    "title": "Step Title",
    "file": "path/to/file (must exist in tree)",
    "description": "Explanation of why this file is important and what it does."
  }
]
`, readmeContent, treeContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating tour itinerary...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 3. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var steps []TourStep
	if err := json.Unmarshal([]byte(jsonStr), &steps); err != nil {
		return fmt.Errorf("failed to parse itinerary: %w\nRaw response: %s", err, resp)
	}

	if len(steps) == 0 {
		return fmt.Errorf("agent generated an empty itinerary")
	}

	// 4. Start TUI
	fmt.Fprintln(cmd.OutOrStdout(), "🚀 Starting tour...")

	m := &TourModel{
		steps: steps,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tour failed: %w", err)
	}

	return nil
}
