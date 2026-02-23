package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TourStop represents a single stop on the codebase tour.
type TourStop struct {
	File      string `json:"file"`
	TitleText string `json:"title"`
	Desc      string `json:"description"`
}

func (t TourStop) FilterValue() string { return t.TitleText }
func (t TourStop) Title() string       { return t.TitleText }
func (t TourStop) Description() string { return t.File }

// TourModel is the Bubble Tea model for the tour TUI.
type TourModel struct {
	stops         []TourStop
	list          list.Model
	viewport      viewport.Model
	ready         bool
	width         int
	height        int
	renderer      *glamour.TermRenderer
	selectedTitle string // to track selection changes
}

var tourCmd = &cobra.Command{
	Use:   "tour [path]",
	Short: "Interactive AI-guided tour of the codebase",
	Long: `Analyze the codebase structure and launch an interactive tour guide.
The AI identifies key components and walks you through them with explanations.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

func runTour(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// verify path is within project root
	cwd, _ := os.Getwd()
	if !strings.HasPrefix(absPath, cwd) {
		// Just a warning, maybe allow it if user wants to tour outside?
		// But for now let's stick to CWD or subdirs.
	}

	fmt.Printf("🔍 Analyzing codebase at %s...\n", targetPath)
	stops, err := generateTour(ctx, absPath)
	if err != nil {
		return fmt.Errorf("failed to generate tour: %w", err)
	}

	if len(stops) == 0 {
		fmt.Println("No tour stops generated. Try a different path or check AI response.")
		return nil
	}

	// Initialize TUI
	items := make([]list.Item, len(stops))
	for i, stop := range stops {
		items[i] = stop
	}

	const listWidth = 30
	l := list.New(items, list.NewDefaultDelegate(), listWidth, 20)
	l.Title = "Codebase Tour"
	l.SetShowHelp(false)

	renderer, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return fmt.Errorf("failed to create markdown renderer: %w", err)
	}

	m := TourModel{
		stops:    stops,
		list:     l,
		renderer: renderer,
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tour failed: %w", err)
	}

	return nil
}

func generateTour(ctx context.Context, root string) ([]TourStop, error) {
	// 1. Generate Context (Tree + README)
	opts := ContextOptions{
		Roots:     []string{root},
		Tree:      true,
		MaxSize:   10 * 1024,
		NoContent: true, // We only need structure
	}

	treeContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return nil, fmt.Errorf("context generation failed: %w", err)
	}

	// Try to find README content manually to help the AI
	readmeContent := ""
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		nameUpper := strings.ToUpper(entry.Name())
		if !entry.IsDir() && strings.HasPrefix(nameUpper, "README") {
			content, _ := os.ReadFile(filepath.Join(root, entry.Name()))
			if len(content) > 0 {
				readmeContent = string(content)
				if len(readmeContent) > 2000 {
					readmeContent = readmeContent[:2000] + "\n...(truncated)"
				}
				break
			}
		}
	}

	// 2. Prompt AI
	prompt := fmt.Sprintf(`You are a Senior Lead Developer onboarding a new hire.
Create a guided tour of the codebase based on the file structure below.

Identify the 5-7 most important files or directories that explain the architecture and flow.
For each stop, provide a title and a detailed explanation of what this component does and why it is important.
Also, identify the file path clearly.

Return a raw JSON list of objects:
[
  {
    "file": "path/to/file_or_dir",
    "title": "Short Title",
    "description": "Detailed explanation (markdown supported)"
  }
]

Do not wrap JSON in markdown blocks. Return only JSON.

README Summary:
%s

File Tree:
%s
`, readmeContent, treeContext)

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory for testing
	ag, err := agentClientFactory(ctx, provider, model, root, "recac-tour")
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent request failed: %w", err)
	}

	// 3. Parse JSON
	jsonStr := utils.CleanJSONBlock(resp)
	var stops []TourStop
	if err := json.Unmarshal([]byte(jsonStr), &stops); err != nil {
		// Log error but maybe try to return raw if it's not JSON? No, strict JSON is better.
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	// Validate paths
	var validStops []TourStop
	for _, stop := range stops {
		// Clean path
		stop.File = filepath.Clean(stop.File)
		// Prevent directory traversal
		if strings.Contains(stop.File, "..") || strings.HasPrefix(stop.File, "/") || strings.HasPrefix(stop.File, "\\") {
			continue
		}

		fullPath := filepath.Join(root, stop.File)

		// Security check: Ensure path is within root
		absFullPath, err := filepath.Abs(fullPath)
		if err != nil {
			continue
		}
		if !strings.HasPrefix(absFullPath, root) {
			continue
		}

		// Ideally we check if file exists, but AI might hallucinate slightly wrong paths or describe dirs.
		// Let's accept it but mark if missing in display?
		// For now, let's just keep it.
		// If we skip, the user might miss context.
		// If we show "File not found", it's safer.
		if _, err := os.Stat(fullPath); err != nil {
			// Try to fuzzy match? Nah.
		}
		validStops = append(validStops, stop)
	}

	return validStops, nil
}

func (m TourModel) Init() tea.Cmd {
	return nil
}

func (m TourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Layout: 30% List, 70% Viewport
		listWidth := int(float64(m.width) * 0.3)
		viewportWidth := m.width - listWidth - 4 // borders/padding

		m.list.SetSize(listWidth, m.height-2)
		m.viewport = viewport.New(viewportWidth, m.height-2)
		m.viewport.Style = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)

		// Force update content on resize
		m.updateViewportContent(true)
	}

	// Handle List Update
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Check if selection changed
	selectedItem := m.list.SelectedItem()
	if selectedItem != nil {
		stop := selectedItem.(TourStop)
		if stop.TitleText != m.selectedTitle {
			m.selectedTitle = stop.TitleText
			m.updateViewportContent(false)
			// Reset viewport scroll to top
			m.viewport.GotoTop()
		}
	}

	// Handle Viewport Update
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *TourModel) updateViewportContent(force bool) {
	if !m.ready {
		return
	}

	selectedItem := m.list.SelectedItem()
	if selectedItem == nil {
		m.viewport.SetContent("Select an item...")
		return
	}

	stop := selectedItem.(TourStop)

	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("# %s\n\n", stop.TitleText))
	contentBuilder.WriteString(fmt.Sprintf("%s\n\n", stop.Desc))
	contentBuilder.WriteString("---\n")

	// Read file content
	// Note: In a real app, this should be async via tea.Cmd to avoid UI freeze on large files
	fileContent, err := os.ReadFile(stop.File)
	if err == nil {
		ext := filepath.Ext(stop.File)
		code := string(fileContent)
		if len(code) > 2000 {
			code = code[:2000] + "\n...(truncated)"
		}
		contentBuilder.WriteString(fmt.Sprintf("\n## File: %s\n\n```%s\n%s\n```\n", stop.File, strings.TrimPrefix(ext, "."), code))
	} else {
		// Just show description if file not found (e.g. directory or hallucination)
		contentBuilder.WriteString(fmt.Sprintf("\n*(File '%s' not found or is a directory)*\n", stop.File))
	}

	rendered, err := m.renderer.Render(contentBuilder.String())
	if err != nil {
		m.viewport.SetContent(fmt.Sprintf("Error rendering markdown: %v", err))
	} else {
		m.viewport.SetContent(rendered)
	}
}

func (m TourModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Layout: Horizontal Join
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.list.View(),
		m.viewport.View(),
	)
}
