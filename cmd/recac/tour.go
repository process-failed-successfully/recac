package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	tourRefresh bool
	tourTopic   string
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take a guided tour of the codebase",
	Long: `Generates and presents an interactive tour of the codebase using AI.
It analyzes the project structure and creates a series of slides explaining key components.
The tour is cached in .recac/tour.json and can be refreshed with --refresh.`,
	RunE: runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
	tourCmd.Flags().BoolVar(&tourRefresh, "refresh", false, "Force regeneration of the tour")
	tourCmd.Flags().StringVar(&tourTopic, "topic", "", "Focus the tour on a specific topic (optional)")
}

type TourSlide struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	File        string `json:"file,omitempty"`
}

func runTour(cmd *cobra.Command, args []string) error {
	// 1. Check for cache
	cacheFile := ".recac/tour.json"
	if tourTopic != "" {
		safeTopic := strings.ReplaceAll(tourTopic, " ", "_")
		cacheFile = fmt.Sprintf(".recac/tour_%s.json", safeTopic)
	}

	var slides []TourSlide
	if !tourRefresh {
		if content, err := os.ReadFile(cacheFile); err == nil {
			if err := json.Unmarshal(content, &slides); err == nil {
				// Cache hit
				return startTour(slides)
			}
		}
	}

	// 2. Generate Tour
	var err error
	slides, err = generateTour(cmd.Context(), tourTopic)
	if err != nil {
		return err
	}

	// 3. Save Cache
	if err := os.MkdirAll(".recac", 0755); err != nil {
		// Just warn, don't fail
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to create cache directory: %v\n", err)
	} else {
		if data, err := json.MarshalIndent(slides, "", "  "); err == nil {
			_ = os.WriteFile(cacheFile, data, 0644)
		}
	}

	// 4. Start TUI
	return startTour(slides)
}

func generateTour(ctx context.Context, topic string) ([]TourSlide, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get CWD: %w", err)
	}

	// Generate Context
	opts := ContextOptions{
		Roots:     []string{"."},
		MaxSize:   50 * 1024, // Smaller context for speed
		Tree:      true,
		NoContent: false,
		Ignore:    []string{".recac", "vendor", "node_modules", ".git"},
	}

	fmt.Println("🔍 Analyzing codebase structure...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate context: %w", err)
	}

	// Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := `You are a Senior Software Engineer onboarding a new team member.
Create a guided tour of this codebase.
The tour should consist of 5-10 "slides" that explain the architecture and key components.

Return a JSON array of objects with the following structure:
[
  {
    "title": "Title of the slide",
    "description": "Markdown description of the component/concept.",
    "file": "Path to the most relevant file (optional)"
  }
]

Requirements:
- Start with a high-level overview.
- Cover the main entry points.
- Explain the core business logic.
- Mention key infrastructure/config files.
- If a specific topic is provided, focus on that.

`

	if topic != "" {
		prompt += fmt.Sprintf("\nFOCUS TOPIC: %s\n", topic)
	}

	prompt += fmt.Sprintf("\nCODEBASE CONTEXT:\n%s", codebaseContext)

	fmt.Println("🤖 Generating tour with AI (this may take a moment)...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed: %w", err)
	}

	jsonStr := utils.CleanJSONBlock(resp)
	var slides []TourSlide
	if err := json.Unmarshal([]byte(jsonStr), &slides); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w\nResponse: %s", err, resp)
	}

	return slides, nil
}

// --- TUI Implementation ---

type TourModel struct {
	Slides   []TourSlide
	Index    int
	Viewport viewport.Model
	Ready    bool
	Renderer *glamour.TermRenderer
	Err      error
}

func startTour(slides []TourSlide) error {
	if len(slides) == 0 {
		return fmt.Errorf("no slides generated")
	}

	// Initialize Glamour renderer
	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to initialize markdown renderer: %v\n", err)
	}

	m := TourModel{
		Slides:   slides,
		Index:    0,
		Renderer: renderer,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run tour: %w", err)
	}
	return nil
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
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "right", "n", "l", " ":
			if m.Index < len(m.Slides)-1 {
				m.Index++
				// Reset viewport position
				m.Viewport.GotoTop()
				// Re-render content for new slide
				m.updateViewportContent()
			}
		case "left", "p", "h", "backspace":
			if m.Index > 0 {
				m.Index--
				m.Viewport.GotoTop()
				m.updateViewportContent()
			}
		}

	case tea.WindowSizeMsg:
		headerHeight := 3
		footerHeight := 3
		verticalMarginHeight := headerHeight + footerHeight

		if !m.Ready {
			m.Viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.Viewport.YPosition = headerHeight
			m.Ready = true
			m.updateViewportContent()
		} else {
			m.Viewport.Width = msg.Width
			m.Viewport.Height = msg.Height - verticalMarginHeight
			m.updateViewportContent() // Re-wrap content on resize
		}
	}

	// Handle viewport updates
	m.Viewport, cmd = m.Viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *TourModel) updateViewportContent() {
	slide := m.Slides[m.Index]
	content := fmt.Sprintf("# %s\n\n%s", slide.Title, slide.Description)

	if slide.File != "" {
		content += fmt.Sprintf("\n\n---\n**Relevant File:** `%s`", slide.File)
	}

	if m.Renderer == nil {
		m.Viewport.SetContent(content)
		return
	}

	rendered, err := m.Renderer.Render(content)
	if err != nil {
		m.Viewport.SetContent(fmt.Sprintf("Error rendering markdown: %v\n\n%s", err, content))
	} else {
		m.Viewport.SetContent(rendered)
	}
}

func (m TourModel) View() string {
	if !m.Ready {
		return "\n  Initializing Tour...\n\n"
	}

	header := m.headerView()
	footer := m.footerView()

	return fmt.Sprintf("%s\n%s\n%s", header, m.Viewport.View(), footer)
}

func (m TourModel) headerView() string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Render(fmt.Sprintf("Tour Step %d/%d", m.Index+1, len(m.Slides)))

	line := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Render(strings.Repeat("─", max(0, m.Viewport.Width-lipgloss.Width(title))))

	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m TourModel) footerView() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))

	info := helpStyle.Render("n: next • p: prev • q: quit")
	line := helpStyle.Render(strings.Repeat("─", max(0, m.Viewport.Width-lipgloss.Width(info))))

	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
