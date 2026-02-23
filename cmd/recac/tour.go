package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// TourSlide represents a single step in the tour
type TourSlide struct {
	Title       string `json:"title"`
	Filepath    string `json:"filepath"`
	Description string `json:"description"` // Markdown content
}

// Global variables for dependency injection
var (
	tourAgentFactory = agent.NewAgent
	tourFile         = ".recac/tour.json"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take an interactive tour of the codebase",
	Long: `Starts an interactive tour of the codebase, guided by AI.
If a tour has not been generated yet, it will analyze the repository structure
and generate a guided tour covering the most important files and concepts.`,
	RunE: runTour,
}

func init() {
	if rootCmd != nil {
		rootCmd.AddCommand(tourCmd)
	}
}

func runTour(cmd *cobra.Command, args []string) error {
	// 1. Check if tour exists
	slides, err := loadTour()
	if err != nil {
		// 2. Generate if not exists
		fmt.Println("Tour not found. Generating a new tour of the codebase...")
		slides, err = generateTour(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to generate tour: %w", err)
		}
		if err := saveTour(slides); err != nil {
			fmt.Printf("Warning: failed to save tour: %v\n", err)
		}
	}

	if len(slides) == 0 {
		return fmt.Errorf("tour is empty")
	}

	// 3. Start TUI
	p := tea.NewProgram(initialModel(slides), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running tour: %w", err)
	}
	return nil
}

func loadTour() ([]TourSlide, error) {
	data, err := os.ReadFile(tourFile)
	if err != nil {
		return nil, err
	}
	var slides []TourSlide
	if err := json.Unmarshal(data, &slides); err != nil {
		return nil, err
	}
	return slides, nil
}

func saveTour(slides []TourSlide) error {
	if err := os.MkdirAll(filepath.Dir(tourFile), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(slides, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(tourFile, data, 0644)
}

func generateTour(ctx context.Context) ([]TourSlide, error) {
	// Initialize Agent
	// Read from config or env
	provider := os.Getenv("RECAC_AGENT_PROVIDER")
	if provider == "" {
		provider = "mock" // Default to mock for safety if not configured
	}
	model := os.Getenv("RECAC_AGENT_MODEL")
	apiKey := os.Getenv("RECAC_API_KEY")

	// Use factory
	a, err := tourAgentFactory(provider, apiKey, model, ".", "recac-tour")
	if err != nil {
		return nil, fmt.Errorf("failed to init agent: %w", err)
	}

	// Read file list to give context
	files, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, f := range files {
		fileNames = append(fileNames, f.Name())
	}

	prompt := fmt.Sprintf(`Analyze the current directory structure and files. Create a guided tour of this codebase for a new developer.
Return a JSON array of objects with keys: 'title', 'filepath', and 'description'.
The description should be valid Markdown explaining the file's purpose.
The tour should start with the README, then cover key configuration files, then the main entry points, and finally important internal packages.
Limit to 5-10 most important files.

Current files in root: %s

Example JSON format:
[
  {
    "title": "Project Overview",
    "filepath": "README.md",
    "description": "# Project Overview\nThis file contains..."
  }
]
IMPORTANT: Return ONLY the JSON array. Do not include markdown code blocks like `+"```json"+` or `+"```"+`.`, strings.Join(fileNames, ", "))

	resp, err := a.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Clean up response if it contains markdown code blocks despite instructions
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```json") {
		resp = strings.TrimPrefix(resp, "```json")
		resp = strings.TrimSuffix(resp, "```")
	} else if strings.HasPrefix(resp, "```") {
		resp = strings.TrimPrefix(resp, "```")
		resp = strings.TrimSuffix(resp, "```")
	}

	var slides []TourSlide
	if err := json.Unmarshal([]byte(resp), &slides); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
	}

	return slides, nil
}

// TUI Model

type tourModel struct {
	slides   []TourSlide
	current  int
	viewport viewport.Model
	ready    bool
	renderer *glamour.TermRenderer
}

func initialModel(slides []TourSlide) tourModel {
	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	return tourModel{
		slides:   slides,
		current:  0,
		renderer: r,
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
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "right", "l", "n", " ":
			if m.current < len(m.slides)-1 {
				m.current++
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
			}
		case "left", "h", "p":
			if m.current > 0 {
				m.current--
				m.viewport.SetContent(m.renderContent())
				m.viewport.GotoTop()
			}
		}
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.renderContent())
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m tourModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m tourModel) headerView() string {
	title := m.slides[m.current].Title
	filepath := m.slides[m.current].Filepath
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Padding(0, 1)
	fileStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Padding(0, 1)
	return fmt.Sprintf("\n%s %s", style.Render(title), fileStyle.Render("("+filepath+")"))
}

func (m tourModel) footerView() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Padding(0, 1)
	return style.Render(fmt.Sprintf("%d/%d • q: quit • ←/h: prev • →/l: next", m.current+1, len(m.slides)))
}

func (m tourModel) renderContent() string {
	slide := m.slides[m.current]
	content := slide.Description

	if m.renderer != nil {
		rendered, err := m.renderer.Render(content)
		if err == nil {
			return rendered
		}
	}
	return content
}
