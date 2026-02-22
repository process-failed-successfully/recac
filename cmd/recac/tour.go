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
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	tourFocus string
	tourLimit int
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take an interactive, AI-guided tour of the codebase",
	Long: `Generates a guided tour of the codebase using AI.
It analyzes the project structure and creates an itinerary of key files and concepts,
presented in an interactive terminal interface.`,
	RunE: runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
	tourCmd.Flags().StringVarP(&tourFocus, "focus", "f", ".", "Focus the tour on a specific directory")
	tourCmd.Flags().IntVarP(&tourLimit, "limit", "l", 7, "Number of tour stops to generate")
}

type TourStop struct {
	TitleText       string `json:"title"`
	DescriptionText string `json:"description"`
	File            string `json:"file"`
}

// Implement list.Item interface
func (t TourStop) Title() string       { return t.TitleText }
func (t TourStop) Description() string { return t.DescriptionText }
func (t TourStop) FilterValue() string { return t.TitleText }

type TourModel struct {
	list     list.Model
	viewport viewport.Model
	stops    []TourStop
	ready    bool
	active   bool // true if focus is on viewport (file content)
	err      error
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
	case tea.WindowSizeMsg:
		h, v := localDocStyle.GetFrameSize()
		m.list.SetSize(msg.Width/3-h, msg.Height-v)
		m.viewport.Width = msg.Width*2/3 - h
		m.viewport.Height = msg.Height - v
		m.ready = true

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "tab", "enter":
			if m.active {
				m.active = false
			} else {
				m.active = true
				// Load file content when focusing
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					stop := selectedItem.(TourStop)
					content, err := os.ReadFile(stop.File)
					if err != nil {
						m.viewport.SetContent(fmt.Sprintf("Error reading file: %v", err))
					} else {
						m.viewport.SetContent(string(content))
					}
				}
			}
		}
	}

	if m.active {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
		// Update viewport preview on selection change
		if m.list.SelectedItem() != nil {
			stop := m.list.SelectedItem().(TourStop)
			content, err := os.ReadFile(stop.File)
			if err == nil {
				m.viewport.SetContent(string(content))
			} else {
				m.viewport.SetContent(fmt.Sprintf("Error reading file: %v", err))
			}
		}
	}

	return m, tea.Batch(cmds...)
}

var localDocStyle = lipgloss.NewStyle().Margin(1, 2)

func (m TourModel) View() string {
	if !m.ready {
		return "\n  Initializing...\n\n"
	}

	listView := m.list.View()
	viewportView := m.viewport.View()

	// Add border to active pane
	if m.active {
		viewportView = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Render(viewportView)
		listView = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Render(listView)
	} else {
		listView = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).Render(listView)
		viewportView = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Render(viewportView)
	}

	return localDocStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, listView, viewportView))
}

func runTour(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	stops, err := generateTourStops(ctx, cwd, tourFocus, tourLimit)
	if err != nil {
		return err
	}

	// 5. Start TUI
	items := make([]list.Item, len(stops))
	for i, s := range stops {
		items[i] = s
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Codebase Tour"

	m := TourModel{
		list:     l,
		viewport: viewport.New(0, 0),
		stops:    stops,
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run tour: %w", err)
	}

	return nil
}

func generateTourStops(ctx context.Context, cwd string, focus string, limit int) ([]TourStop, error) {
	// 1. Generate Context (Lightweight)
	opts := ContextOptions{
		Roots:     []string{focus},
		MaxSize:   50 * 1024,
		Tree:      true,
		NoContent: false,
	}

	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`You are a senior developer giving a tour of this codebase.
Create a guided itinerary of %d key stops (files or directories) that a new developer should visit to understand the project.
Start with the most important entry points (e.g., main.go, README.md, key packages).

Return the result as a raw JSON list of objects:
[
  {
    "title": "Stop Title",
    "description": "Why this is important.",
    "file": "path/to/file/or/dir"
  }
]

Do not wrap the JSON in markdown code blocks. Just return the raw JSON string.

CODEBASE CONTEXT:
%s`, limit, codebaseContext)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed to generate tour: %w", err)
	}

	// 4. Parse Response
	jsonStr := utils.CleanJSONBlock(resp)
	var stops []TourStop
	if err := json.Unmarshal([]byte(jsonStr), &stops); err != nil {
		// Try to recover if wrapped in markdown
		if start := strings.Index(jsonStr, "["); start != -1 {
			if end := strings.LastIndex(jsonStr, "]"); end != -1 {
				jsonStr = jsonStr[start : end+1]
				if err := json.Unmarshal([]byte(jsonStr), &stops); err != nil {
					return nil, fmt.Errorf("failed to parse agent response: %v\nResponse: %s", err, resp)
				}
			}
		} else {
			return nil, fmt.Errorf("failed to parse agent response: %v\nResponse: %s", err, resp)
		}
	}

	if len(stops) == 0 {
		return nil, fmt.Errorf("agent returned no tour stops")
	}

	// Validate paths
	var validStops []TourStop
	for _, s := range stops {
		cleanPath := filepath.Clean(s.File)
		if _, err := os.Stat(cleanPath); err == nil {
			s.File = cleanPath
			validStops = append(validStops, s)
		} else {
			// Try relative to cwd
			relPath := filepath.Join(cwd, s.File)
			if _, err := os.Stat(relPath); err == nil {
				s.File = relPath
				validStops = append(validStops, s)
			}
		}
	}

	if len(validStops) == 0 {
		return nil, fmt.Errorf("no valid files found in tour stops")
	}

	return validStops, nil
}
