package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var tourCmd = &cobra.Command{
	Use:   "tour",
	Short: "Take an interactive AI-guided tour of the codebase",
	Long: `Starts an interactive TUI (Terminal User Interface) that guides you through the key components of the codebase.
If a tour itinerary doesn't exist, it uses AI to generate one based on the project structure and README.`,
	RunE: runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

// Data Structures

type TourItem struct {
	TitleText       string `json:"title"`
	File            string `json:"file"`
	DescriptionText string `json:"description"`
}

func (t TourItem) FilterValue() string { return t.TitleText }
func (t TourItem) Title() string       { return t.TitleText }
func (t TourItem) Description() string { return t.DescriptionText }

type TourItinerary struct {
	ProjectName string     `json:"project_name"`
	Overview    string     `json:"overview"`
	Steps       []TourItem `json:"steps"`
}

// TUI Model

type TourModel struct {
	list        list.Model
	viewport    viewport.Model
	items       []TourItem
	selected    *TourItem
	ready       bool
	viewState   int // 0: list, 1: details
	content     string
	width       int
	height      int
}

const (
	viewList    = 0
	viewDetails = 1
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF7DB")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)
	descStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0"))
)

func (m TourModel) Init() tea.Cmd {
	return nil
}

func (m TourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		if m.viewState == viewList {
			if msg.String() == "enter" {
				// Select item
				if i, ok := m.list.SelectedItem().(TourItem); ok {
					m.selected = &i
					m.viewState = viewDetails

					// Load file content
					content, err := os.ReadFile(i.File)
					if err != nil {
						m.content = fmt.Sprintf("Error reading file %s: %v", i.File, err)
					} else {
						m.content = fmt.Sprintf("# %s\n\n> %s\n\n```\n%s\n```", i.TitleText, i.DescriptionText, string(content))
					}

					m.viewport.SetContent(m.content)
					m.viewport.GotoTop()
				}
			}
		} else if m.viewState == viewDetails {
			if msg.String() == "esc" || msg.String() == "q" {
				m.viewState = viewList
				return m, nil
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

		m.viewport.Width = msg.Width - h
		m.viewport.Height = msg.Height - v
	}

	if m.viewState == viewList {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m TourModel) View() string {
	if m.viewState == viewList {
		return docStyle.Render(m.list.View())
	}

	return docStyle.Render(m.viewport.View() + "\n\n(Press esc or q to return)")
}

// Logic

func runTour(cmd *cobra.Command, args []string) error {
	tourFile := filepath.Join(".recac", "tour.json")

	// Check if tour exists
	var itinerary TourItinerary
	if _, err := os.Stat(tourFile); os.IsNotExist(err) {
		fmt.Fprintln(cmd.OutOrStdout(), "No tour found. Generating one using AI (this may take a minute)...")
		itinerary, err = generateItinerary(cmd.Context())
		if err != nil {
			return fmt.Errorf("failed to generate itinerary: %w", err)
		}

		// Save it
		if err := os.MkdirAll(".recac", 0755); err != nil {
			return err
		}
		data, _ := json.MarshalIndent(itinerary, "", "  ")
		if err := os.WriteFile(tourFile, data, 0644); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Tour generated and saved to .recac/tour.json")
	} else {
		data, err := os.ReadFile(tourFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &itinerary); err != nil {
			return fmt.Errorf("failed to parse tour file: %w", err)
		}
	}

	// Initialize TUI
	items := make([]list.Item, len(itinerary.Steps))
	for i, step := range itinerary.Steps {
		items[i] = step
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = fmt.Sprintf("Tour: %s", itinerary.ProjectName)

	m := TourModel{
		list:      l,
		items:     itinerary.Steps,
		viewState: viewList,
		viewport:  viewport.New(0, 0),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func generateItinerary(ctx context.Context) (TourItinerary, error) {
	// 1. Gather Context
	opts := ContextOptions{
		Roots:     []string{"."},
		MaxSize:   50 * 1024,
		Tree:      true,
		NoContent: false,
		Ignore:    []string{"go.sum", "go.mod", "test_output.txt"},
	}

	// Limit to specific files to save tokens/time if needed, but for a tour we want structure + key files.
	// We'll rely on GenerateCodebaseContext to be smart (it has limits).
	// But let's verify we at least get README.md
	if _, err := os.Stat("README.md"); err == nil {
		opts.Roots = append(opts.Roots, "README.md")
	}

	contextStr, err := GenerateCodebaseContext(opts)
	if err != nil {
		return TourItinerary{}, err
	}

	// 2. Prepare Agent
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return TourItinerary{}, err
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`You are a Senior Technical Lead onboarding a new developer.
Create a "Codebase Tour" itinerary for this project.

The tour should consist of 5-10 key files that explain the architecture and main functionality.
Start with high-level entry points and move to core logic.

Context:
%s

Return the result as a raw JSON object (no markdown) with this structure:
{
  "project_name": "Name of project",
  "overview": "Short overview",
  "steps": [
    {
      "title": "Entry Point",
      "file": "cmd/app/main.go",
      "description": "Explains how the app starts..."
    },
    ...
  ]
}
Ensure the "file" paths exist in the context provided.
`, contextStr)

	// 4. Call Agent
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return TourItinerary{}, err
	}

	// 5. Parse
	jsonStr := utils.CleanJSONBlock(resp)
	var itinerary TourItinerary
	if err := json.Unmarshal([]byte(jsonStr), &itinerary); err != nil {
		return TourItinerary{}, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, resp)
	}

	return itinerary, nil
}
