package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// TourStop represents a stop in the codebase tour.
type TourStop struct {
	Path        string `json:"path"`
	Desc        string `json:"description"`
}

// Implement list.Item interface (DefaultItem)
func (i TourStop) Title() string       { return i.Path }
func (i TourStop) Description() string { return i.Desc }
func (i TourStop) FilterValue() string { return i.Path }

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take an AI-guided tour of the codebase",
	Long:  `Analyzes the codebase structure and generates an interactive tour of the most important files and components using AI.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTour(cmd)
	},
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

func runTour(cmd *cobra.Command) error {
	ctx := context.Background()

	// Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Initial Model
	m := initialModel(ctx, ag)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running tour: %w", err)
	}
	return nil
}

// GenerateTourStops asks the AI to identify key files.
func GenerateTourStops(ctx context.Context, ag agent.Agent) ([]TourStop, error) {
	// 1. Get Project Structure (Tree only to save tokens)
	contextStr, err := GenerateCodebaseContext(ContextOptions{
		Roots:     []string{"."},
		Tree:      true,
		NoContent: true,
		Ignore:    []string{".git", "node_modules", "dist", "vendor", ".recac", "tmp"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate context: %w", err)
	}

	// 2. Prompt AI
	prompt := fmt.Sprintf(`You are a senior software architect.
I need you to create a "Guided Tour" of this codebase for a new developer.
Analyze the following file tree and identify the 5-7 most critical files or directories that explain the system architecture and core logic.

Codebase Structure:
'''
%s
'''

Return a JSON list of objects with "path" and "description" fields.
The "path" must be the relative path to the file or directory.
The "description" should be a short summary (1 sentence) of why this file is important.

Example JSON:
[
  {"path": "cmd/main.go", "description": "The entry point of the application."},
  {"path": "internal/core/engine.go", "description": "Contains the main business logic loop."}
]

Return ONLY the JSON.
`, contextStr)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed: %w", err)
	}

	// 3. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var stops []TourStop
	if err := json.Unmarshal([]byte(jsonStr), &stops); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w\nResponse: %s", err, resp)
	}

	return stops, nil
}

// --- Bubble Tea Model ---

type tourModel struct {
	list     list.Model
	viewport viewport.Model
	stops    []TourStop
	agent    agent.Agent
	ctx      context.Context
	ready    bool
	loading  bool
	err      error
	content  string
}

func initialModel(ctx context.Context, ag agent.Agent) tourModel {
	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Codebase Tour"
	l.SetShowHelp(false)
	l.DisableQuitKeybindings() // We handle ctrl+c manually

	return tourModel{
		list:    l,
		agent:   ag,
		ctx:     ctx,
		loading: true, // Start loading immediately
	}
}

type stopsMsg []TourStop
type contentMsg string
type errMsg error

func (m tourModel) Init() tea.Cmd {
	return func() tea.Msg {
		stops, err := GenerateTourStops(m.ctx, m.agent)
		if err != nil {
			return errMsg(err)
		}
		return stopsMsg(stops)
	}
}

func (m tourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	// Handle global keys
	if k, ok := msg.(tea.KeyMsg); ok {
		if k.String() == "ctrl+c" || k.String() == "q" {
			return m, tea.Quit
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		// Split screen: 1/3 list, 2/3 viewport
		totalWidth := msg.Width - h
		listWidth := totalWidth / 3
		viewWidth := totalWidth - listWidth - 2 // -2 for gap

		m.list.SetSize(listWidth, msg.Height-v)
		m.viewport = viewport.New(viewWidth, msg.Height-v-3) // -3 for header/footer
		m.ready = true

	case stopsMsg:
		m.loading = false
		m.stops = msg
		items := make([]list.Item, len(msg))
		for i, stop := range msg {
			items[i] = stop
		}
		m.list.SetItems(items)

		// Load content of first item if available
		if len(m.stops) > 0 {
			cmds = append(cmds, m.loadContentCmd(m.stops[0].Path))
		}

	case contentMsg:
		m.content = string(msg)
		m.viewport.SetContent(m.content)

	case errMsg:
		m.err = msg
		m.loading = false
	}

	// Update list
	m.list, cmd = m.list.Update(msg)
	cmds = append(cmds, cmd)

	// Check for selection change via navigation keys
	if k, ok := msg.(tea.KeyMsg); ok && !m.loading && len(m.stops) > 0 {
		key := k.String()
		if key == "up" || key == "down" || key == "j" || key == "k" {
			if i := m.list.SelectedItem(); i != nil {
				stop := i.(TourStop)
				cmds = append(cmds, m.loadContentCmd(stop.Path))
			}
		}
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m tourModel) loadContentCmd(path string) tea.Cmd {
	return func() tea.Msg {
		// Sanitize path
		cwd, err := os.Getwd()
		if err != nil {
			return contentMsg(fmt.Sprintf("Error getting cwd: %v", err))
		}
		absPath, err := filepath.Abs(filepath.Join(cwd, path))
		if err != nil {
			return contentMsg(fmt.Sprintf("Error resolving path: %v", err))
		}
		if absPath != cwd && !strings.HasPrefix(absPath, cwd+string(os.PathSeparator)) {
			return contentMsg(fmt.Sprintf("Security Error: path outside of project root: %s", path))
		}

		content, err := os.ReadFile(absPath)
		if err != nil {
			return contentMsg(fmt.Sprintf("Error reading file %s: %v", path, err))
		}
		return contentMsg(string(content))
	}
}

func (m tourModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.\n", m.err)
	}
	if m.loading {
		return "\n  Generating Tour... (This may take a moment)\n"
	}
	if !m.ready {
		return "\n  Initializing...\n"
	}

	return docStyle.Render(
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			listStyle.Render(m.list.View()),
			viewStyle.Render(
				fmt.Sprintf("## %s\n\n%s",
					m.selectedTitle(),
					m.viewport.View(),
				),
			),
		),
	)
}

func (m tourModel) selectedTitle() string {
	if i := m.list.SelectedItem(); i != nil {
		return i.(TourStop).Path
	}
	return ""
}

// Styles
var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	listStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1).
			MarginRight(1)
	viewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(0, 1)
)
