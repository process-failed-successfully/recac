package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	tourStyle     = lipgloss.NewStyle().Margin(1, 2)
	tourListStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, true, false, false).
			BorderForeground(lipgloss.Color("240")).
			MarginRight(2)
	tourContentStyle = lipgloss.NewStyle().
				Padding(0, 1)
	tourTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true).
			Padding(0, 1)
	tourExplanationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true).
				Padding(1, 1)
)

type TourStep struct {
	Path    string `json:"path"`
	Summary string `json:"summary"`
}

// Implement list.Item interface
func (s TourStep) FilterValue() string { return s.Path }
func (s TourStep) Title() string       { return s.Path }
func (s TourStep) Description() string { return s.Summary }

type TourPlan struct {
	Steps []TourStep `json:"steps"`
}

type TourModel struct {
	list        list.Model
	viewport    viewport.Model
	spinner     spinner.Model
	steps       []TourStep
	content     string
	explanation string
	loading     bool
	ready       bool
	agent       agent.Agent
	root        string
	width       int
	height      int
	err         error
}

type tourLoadedMsg struct {
	steps []TourStep
}

type contentLoadedMsg struct {
	content string
}

type explanationLoadedMsg struct {
	explanation string
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

var tourCmd = &cobra.Command{
	Use:   "tour [path]",
	Short: "Interactive codebase tour",
	Long:  `Generates an interactive, guided tour of the codebase using AI. It identifies key files and provides explanations.`,
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTour,
}

func init() {
	rootCmd.AddCommand(tourCmd)
}

func runTour(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-tour")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Initial Model
	m := TourModel{
		agent:   ag,
		root:    root,
		loading: true,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tour failed: %w", err)
	}
	return nil
}

func (m TourModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		generateTourPlanCmd(m.agent, m.root),
	)
}

func (m TourModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		// Disable interaction while loading to prevent state desync
		if m.loading {
			return m, nil
		}
		if msg.String() == "enter" && m.ready {
			selectedItem := m.list.SelectedItem()
			if selectedItem != nil {
				step := selectedItem.(TourStep)
				m.loading = true
				return m, tea.Batch(
					m.spinner.Tick,
					loadFileContentCmd(step.Path),
					fetchExplanationCmd(m.agent, step.Path),
				)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout: List takes 30%, Viewport takes 70%
		listWidth := int(float64(msg.Width) * 0.3)
		viewportWidth := msg.Width - listWidth - 4 // borders/margin

		m.list.SetSize(listWidth, msg.Height-2)
		m.viewport.Width = viewportWidth
		m.viewport.Height = msg.Height - 2

	case tourLoadedMsg:
		m.loading = false
		m.ready = true
		m.steps = msg.steps

		items := make([]list.Item, len(m.steps))
		for i, s := range m.steps {
			items[i] = s
		}
		m.list = list.New(items, list.NewDefaultDelegate(), 0, 0)
		m.list.Title = "Codebase Tour"
		m.list.SetSize(int(float64(m.width)*0.3), m.height-2)

		// Initialize viewport
		m.viewport = viewport.New(m.width-int(float64(m.width)*0.3)-4, m.height-2)

		// Load first item automatically
		if len(m.steps) > 0 {
			m.loading = true // Set loading true for initial fetch
			return m, tea.Batch(
				m.spinner.Tick,
				loadFileContentCmd(m.steps[0].Path),
				fetchExplanationCmd(m.agent, m.steps[0].Path),
			)
		}

	case contentLoadedMsg:
		m.content = msg.content
		m.renderViewport()

	case explanationLoadedMsg:
		m.explanation = msg.explanation
		m.loading = false
		m.renderViewport()

	case errMsg:
		m.err = msg.err
		m.loading = false
		return m, tea.Quit
	}

	var cmd tea.Cmd

	// Always update spinner if loading
	if m.loading {
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Only update list and viewport if ready and NOT loading (or allow viewport scroll?)
	// Allowing viewport scroll while explanation loads is nice, but let's keep it simple.
	// If loading, we blocked key inputs anyway (except quit).
	if m.ready && !m.loading {
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *TourModel) renderViewport() {
	var sb strings.Builder
	sb.WriteString(tourTitleStyle.Render("File Content"))
	sb.WriteString("\n")
	sb.WriteString(tourContentStyle.Render(m.content))
	sb.WriteString("\n")
	sb.WriteString(tourTitleStyle.Render("AI Explanation"))
	sb.WriteString("\n")
	sb.WriteString(tourExplanationStyle.Render(m.explanation))

	m.viewport.SetContent(sb.String())
}

func (m TourModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	if !m.ready {
		return fmt.Sprintf("\n\n   %s Generating tour plan... please wait.\n\n", m.spinner.View())
	}

	// Use normal view but maybe show spinner in title?
	view := lipgloss.JoinHorizontal(lipgloss.Top,
			tourListStyle.Render(m.list.View()),
			tourContentStyle.Render(m.viewport.View()),
		)

	if m.loading {
		// Overlay or append spinner
		return fmt.Sprintf("%s\n\n%s Loading...", view, m.spinner.View())
	}

	return tourStyle.Render(view)
}

func generateTourPlanCmd(ag agent.Agent, root string) tea.Cmd {
	return func() tea.Msg {
		// 1. Generate Tree Context
		opts := ContextOptions{
			Roots: []string{root},
			Tree:  true,
		}
		treeContext, err := GenerateCodebaseContext(opts)
		if err != nil {
			return errMsg{err}
		}

		// 2. Ask Agent
		prompt := fmt.Sprintf(`Analyze the following file tree and create a guided tour of the 5-10 most important files or directories for a new developer.
Return a JSON object with a single key "steps" which is an array of objects with "path" and "summary" keys.
"path" should be the relative file path. "summary" should be a 1-sentence description.

File Tree:
%s`, treeContext)

		resp, err := ag.Send(context.Background(), prompt)
		if err != nil {
			return errMsg{err}
		}

		// 3. Parse JSON
		var plan TourPlan
		// Clean markdown blocks
		cleaned := utils.CleanJSONBlock(resp)
		if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
			return errMsg{fmt.Errorf("failed to parse tour plan: %w", err)}
		}

		return tourLoadedMsg{steps: plan.Steps}
	}
}

func loadFileContentCmd(path string) tea.Cmd {
	return func() tea.Msg {
		content, err := os.ReadFile(path)
		if err != nil {
			return contentLoadedMsg{content: fmt.Sprintf("Error reading file: %v", err)}
		}
		// Limit content size for display?
		s := string(content)
		if len(s) > 5000 {
			s = s[:5000] + "\n...(truncated)..."
		}
		return contentLoadedMsg{content: s}
	}
}

func fetchExplanationCmd(ag agent.Agent, path string) tea.Cmd {
	return func() tea.Msg {
		// Read content again (or pass it? efficient to read here again for context)
		content, _ := os.ReadFile(path) // Ignore error as loadFileContentCmd handles it
		s := string(content)
		if len(s) > 2000 {
			s = s[:2000] + "..."
		}

		prompt := fmt.Sprintf(`Explain the following file briefly (3-5 sentences) in the context of the project.
File: %s
Content:
'''
%s
'''`, path, s)

		resp, err := ag.Send(context.Background(), prompt)
		if err != nil {
			return explanationLoadedMsg{explanation: "Failed to fetch explanation."}
		}
		return explanationLoadedMsg{explanation: resp}
	}
}
