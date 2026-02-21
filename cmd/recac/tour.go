package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Interactive codebase tour",
	Long:  `Generates an interactive tour of the codebase using AI, guiding you through key files and concepts.`,
	RunE:  runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

type TourStep struct {
	Title       string `json:"title"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
}

type TourItinerary struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Steps       []TourStep `json:"steps"`
}

type tourModel struct {
	itinerary   *TourItinerary
	currentStep int
	viewport    viewport.Model
	explanation viewport.Model
	spinner     spinner.Model
	loading     bool
	err         error
	width       int
	height      int
	ready       bool
}

func initialTourModel() tourModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return tourModel{
		spinner: s,
		loading: true,
	}
}

func (m tourModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		generateItineraryCmd(),
	)
}

func (m tourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "right", "n", "l":
			if m.currentStep < len(m.itinerary.Steps)-1 {
				m.currentStep++
				m.updateContent()
			}
		case "left", "p", "h":
			if m.currentStep > 0 {
				m.currentStep--
				m.updateContent()
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.ready {
			m.viewport = viewport.New(msg.Width/2-2, msg.Height-10)
			m.explanation = viewport.New(msg.Width/2-2, msg.Height-10)
			m.ready = true
		} else {
			m.viewport.Width = msg.Width/2 - 2
			m.viewport.Height = msg.Height - 10
			m.explanation.Width = msg.Width/2 - 2
			m.explanation.Height = msg.Height - 10
		}

		if !m.loading {
			m.updateContent()
		}

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case itineraryMsg:
		if msg.Err != nil {
			m.err = msg.Err
			m.loading = false
			return m, tea.Quit
		}
		m.itinerary = msg.Itinerary
		m.loading = false
		m.currentStep = 0
		m.updateContent()
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)
	m.explanation, cmd = m.explanation.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *tourModel) updateContent() {
	if m.itinerary == nil || len(m.itinerary.Steps) == 0 {
		return
	}

	step := m.itinerary.Steps[m.currentStep]

	// Sanitize path to prevent directory traversal
	cleanPath := filepath.Clean(step.File)
	if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") || (len(cleanPath) > 1 && cleanPath[1] == ':') {
		m.viewport.SetContent(fmt.Sprintf("Security Warning: File path '%s' is invalid or outside the project root.", step.File))
	} else {
		// Read file content
		content, err := os.ReadFile(cleanPath)
		if err != nil {
			m.viewport.SetContent(fmt.Sprintf("Error reading file %s: %v", step.File, err))
		} else {
			// Highlight line?
			// For simplicity, just show content. In real implementation, use chroma.
			// We can add simple line numbers.
			lines := strings.Split(string(content), "\n")
			var numberedLines []string
			for i, line := range lines {
				prefix := "   "
				if i+1 == step.Line {
					prefix = "-> "
				}
				numberedLines = append(numberedLines, fmt.Sprintf("%4d| %s%s", i+1, prefix, line))
			}

			fullContent := strings.Join(numberedLines, "\n")
			m.viewport.SetContent(fullContent)

			// Try to scroll to line
			// Viewport implementation scrolls by percent or lines.
			// Simple heuristic: set Y offset.
			if step.Line > 0 {
				// Center the line
				targetY := step.Line - (m.viewport.Height / 2)
				if targetY < 0 {
					targetY = 0
				}
				m.viewport.YOffset = targetY
			}
		}
	}

	// Explanation
	expl := fmt.Sprintf("# %s\n\n%s\n\nFile: %s:%d", step.Title, step.Description, step.File, step.Line)
	m.explanation.SetContent(expl)
}

func (m tourModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit.", m.err)
	}

	if m.loading {
		return fmt.Sprintf("\n\n   %s Generating your tour itinerary...\n\n", m.spinner.View())
	}

	if !m.ready {
		return "Initializing..."
	}

	if m.itinerary == nil {
		return "No itinerary generated."
	}

	// Header
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render(fmt.Sprintf("Codebase Tour: %s (%d/%d)", m.itinerary.Title, m.currentStep+1, len(m.itinerary.Steps)))

	// Footer
	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("←/→: Navigate • q: Quit")

	// Split View
	left := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(m.width/2 - 2).
		Render(m.viewport.View())

	right := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(m.width/2 - 2).
		Padding(1).
		Render(m.explanation.View())

	return lipgloss.JoinVertical(lipgloss.Center,
		header,
		lipgloss.JoinHorizontal(lipgloss.Top, left, right),
		footer,
	)
}

type itineraryMsg struct {
	Itinerary *TourItinerary
	Err       error
}

func generateItineraryCmd() tea.Cmd {
	return func() tea.Msg {
		// This runs in a goroutine
		itinerary, err := generateItinerary()
		return itineraryMsg{Itinerary: itinerary, Err: err}
	}
}

func generateItinerary() (*TourItinerary, error) {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// 1. Generate Context (light version)
	opts := ContextOptions{
		Roots:   []string{"."},
		MaxSize: 50 * 1024, // 50KB limit
		Tree:    true,
	}
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return nil, err
	}

	// 2. Ask Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`Generate an interactive tour of the codebase for a new developer.
Identify 5-10 key files or components that explain the architecture and flow.
Return a JSON object with the following structure:
{
  "title": "Tour Title",
  "description": "Short overview",
  "steps": [
    {
      "title": "Step Title",
      "file": "path/to/file.go",
      "line": 1,
      "description": "Explanation of why this file is important and what it does."
    }
  ]
}

Codebase Context:
%s`, codebaseContext)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	jsonStr := utils.CleanJSONBlock(resp)
	var itinerary TourItinerary
	if err := json.Unmarshal([]byte(jsonStr), &itinerary); err != nil {
		return nil, fmt.Errorf("failed to parse itinerary: %w", err)
	}

	return &itinerary, nil
}

func runTour(cmd *cobra.Command, args []string) error {
	p := tea.NewProgram(initialTourModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
