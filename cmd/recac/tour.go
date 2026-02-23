package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take an interactive AI-guided tour of the codebase",
	Long:  `Generates and presents an interactive, step-by-step tour of the current repository using AI.`,
	RunE:  runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

func runTour(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Initial model state
	m := NewTourModel(cwd)

	// Start Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run tour: %w", err)
	}
	return nil
}

// TourModel holds the state of the tour
type TourModel struct {
	cwd        string
	slides     []string
	index      int
	ready      bool
	err        error
	spinner    spinner.Model
	loading    bool
	renderer   *glamour.TermRenderer
	width      int
	height     int
}

// NewTourModel creates a new tour model
func NewTourModel(cwd string) TourModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205")) // Pinkish

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return TourModel{
		cwd:      cwd,
		spinner:  s,
		loading:  true,
		renderer: r,
	}
}

// Init triggers the content generation
func (m TourModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		generateTourContent(m.cwd),
	)
}

// Update handles messages
func (m TourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "right", "n", "enter", "space":
			if m.ready && m.index < len(m.slides)-1 {
				m.index++
			}
		case "left", "p", "backspace":
			if m.ready && m.index > 0 {
				m.index--
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.renderer != nil {
			// Update word wrap based on width, leaving some margin
			_ = m.renderer // glamour renderer is immutable, usually we re-create or just let it be.
			// Actually, NewTermRenderer options are applied at creation.
			// To be responsive, we might need to recreate it, but for now let's stick to default or 80.
			// Better: recreate with new width
			r, _ := glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(msg.Width-4),
			)
			m.renderer = r
		}

	case tourContentMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.slides = msg.slides
		m.ready = true

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

// View renders the UI
func (m TourModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress q to quit.", m.err)
	}

	if m.loading {
		return fmt.Sprintf("\n\n   %s Generating your guided tour of the codebase...\n\n   (This may take a moment as the AI analyzes the structure)\n", m.spinner.View())
	}

	if !m.ready || len(m.slides) == 0 {
		return "No tour content generated. Press q to quit."
	}

	slide := m.slides[m.index]
	rendered, err := m.renderer.Render(slide)
	if err != nil {
		rendered = slide // Fallback to raw markdown
	}

	// Footer
	footer := fmt.Sprintf("\n\nSlide %d of %d | Next: Right/Enter | Prev: Left | Quit: q", m.index+1, len(m.slides))
	return rendered + footer
}

// Message types
type tourContentMsg struct {
	slides []string
	err    error
}

// generateTourContent creates the command to fetch content
func generateTourContent(cwd string) tea.Cmd {
	return func() tea.Msg {
		// 1. Analyze structure
		structure, err := getRepoStructure(cwd)
		if err != nil {
			return tourContentMsg{err: err}
		}

		// 2. Construct Prompt
		prompt := fmt.Sprintf(`You are an expert software architect and guide.
I need you to generate a guided tour of this codebase for a new developer.
The tour should consist of 5 to 10 slides.

Repository Structure:
%s

Instructions:
1. The first slide should be a "Welcome" slide, explaining what the project seems to be based on the structure.
2. Subsequent slides should walk through key directories (e.g., cmd, pkg, internal, src) and important files.
3. Explain the purpose of each component.
4. The last slide should be "Next Steps" (how to run, test, or contribute).
5. Output MUST be a valid JSON array of strings. Each string is the Markdown content of one slide.
6. Use Markdown formatting (headers, bold, lists, code blocks) to make it pretty.
7. Do NOT include standard markdown code block fences around the JSON (like `+"```json"+` ... `+"```"+`). Just the raw JSON array.

Example Output:
[
  "# Welcome to Project X\n\nThis is a Go project...",
  "# The cmd Directory\n\nContains entry points..."
]
`, structure)

		// 3. Call Agent
		ctx := context.Background()
		provider := viper.GetString("provider")
		model := viper.GetString("model")
		// Fallback to default if not set (though viper usually handles defaults)
		if provider == "" {
			provider = "openrouter" // Default or maybe mock?
		}

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
		if err != nil {
			return tourContentMsg{err: fmt.Errorf("failed to create agent: %w", err)}
		}

		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return tourContentMsg{err: fmt.Errorf("agent failed: %w", err)}
		}

		// 4. Parse JSON
		var slides []string
		// Clean up response if it contains markdown blocks
		cleaned := strings.TrimSpace(resp)
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")

		if err := json.Unmarshal([]byte(cleaned), &slides); err != nil {
			// Fallback: try to split by some delimiter or just show raw response
			// Maybe the agent didn't return JSON.
			// Let's create a single slide with the raw response.
			slides = []string{"# Tour Generation Issue\n\nThe agent did not return valid JSON, but here is the raw output:\n\n" + resp}
		}

		return tourContentMsg{slides: slides}
	}
}

// getRepoStructure returns a string representation of the file tree
func getRepoStructure(root string) (string, error) {
	var sb strings.Builder
	maxDepth := 2 // Don't go too deep

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if path == root {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		parts := strings.Split(rel, string(os.PathSeparator))
		depth := len(parts)

		// Skip hidden files/dirs (like .git, .idea)
		if strings.HasPrefix(parts[depth-1], ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		indent := strings.Repeat("  ", depth-1)
		sb.WriteString(fmt.Sprintf("%s%s", indent, parts[depth-1]))
		if info.IsDir() {
			sb.WriteString("/")
		}
		sb.WriteString("\n")

		return nil
	})

	return sb.String(), err
}
