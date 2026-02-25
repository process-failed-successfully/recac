package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// -- Data Structures --

type DraftQuestion struct {
	Question string `json:"question"`
	Context  string `json:"context,omitempty"`
}

type DraftSpec struct {
	ProjectName string
	Pitch       string
	Answers     []string
}

// -- Command --

var architectDraftCmd = &cobra.Command{
	Use:   "draft",
	Short: "Interactively draft an app_spec.txt",
	Long:  "Launch an interactive wizard to brainstorm and draft a system specification with AI assistance.",
	RunE:  runArchitectDraft,
}

func init() {
	architectCmd.AddCommand(architectDraftCmd)
	architectDraftCmd.Flags().String("out", "app_spec.txt", "Output file path")
}

func runArchitectDraft(cmd *cobra.Command, args []string) error {
	outPath, _ := cmd.Flags().GetString("out")

	// Init Agent
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Use factory for testability
	ag, err := agentClientFactory(ctx, provider, model, ".", "recac-architect-draft")
	if err != nil {
		return fmt.Errorf("failed to init agent: %w", err)
	}

	// Start TUI
	p := tea.NewProgram(initialDraftModel(ag, outPath), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running draft wizard: %w", err)
	}
	return nil
}

// -- TUI Model --

type DraftState int

const (
	StateDraftProjectName DraftState = iota
	StateDraftPitch
	StateDraftThinkingQuestions
	StateDraftAnsweringQuestions
	StateDraftThinkingSpec
	StateDraftReview
	StateDraftDone
)

type draftModel struct {
	agent   agent.Agent
	outPath string
	state   DraftState

	projectNameInput textinput.Model
	pitchInput       textarea.Model
	answerInput      textarea.Model

	questions []DraftQuestion
	answers   []string
	currQIdx  int

	finalSpec string
	err       error

	width, height int
	viewport      viewport.Model
	renderer      *glamour.TermRenderer
}

func initialDraftModel(ag agent.Agent, outPath string) draftModel {
	ti := textinput.New()
	ti.Placeholder = "My Awesome App"
	ti.Focus()
	ti.CharLimit = 100

	ta := textarea.New()
	ta.Placeholder = "Describe your idea..."
	ta.CharLimit = 2000
	ta.SetHeight(5)
	ta.ShowLineNumbers = false

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return draftModel{
		agent:            ag,
		outPath:          outPath,
		state:            StateDraftProjectName,
		projectNameInput: ti,
		pitchInput:       ta,
		answerInput:      ta,
		renderer:         r,
	}
}

// -- Messages --

type questionsMsg struct {
	questions []DraftQuestion
	err       error
}

type specMsg struct {
	spec string
	err  error
}

// -- Commands --

func generateDraftQuestionsCmd(m draftModel) tea.Cmd {
	return func() tea.Msg {
		prompt := fmt.Sprintf(`You are a software architect.
I have an idea for a project: "%s".
Description: "%s".

Generate 3 clarifying questions to better understand the requirements (e.g., target audience, tech stack preference, scale, key features).
Return ONLY a JSON array of objects with "question" (string) and "context" (string, optional).
Example:
[
  {"question": "Who is the target audience?", "context": "To determine UI/UX needs"},
  {"question": "Do you have a preferred tech stack?", "context": "Go/Python/Node?"}
]`, m.projectNameInput.Value(), m.pitchInput.Value())

		resp, err := m.agent.Send(context.Background(), prompt)
		if err != nil {
			return questionsMsg{err: err}
		}

		var qs []DraftQuestion
		if err := parseJSONList(resp, &qs); err != nil {
			return questionsMsg{err: fmt.Errorf("failed to parse questions: %w", err)}
		}
		return questionsMsg{questions: qs}
	}
}

func generateDraftSpecCmd(m draftModel) tea.Cmd {
	return func() tea.Msg {
		// Build Q&A transcript
		var transcript string
		for i, q := range m.questions {
			ans := ""
			if i < len(m.answers) {
				ans = m.answers[i]
			}
			transcript += fmt.Sprintf("Q: %s\nA: %s\n\n", q.Question, ans)
		}

		prompt := fmt.Sprintf(`You are a software architect.
Create a comprehensive application specification (app_spec.txt) based on the following interview.

Project: %s
Pitch: %s

Interview Transcript:
%s

The spec should include:
1. High-level summary
2. Core Features (Functional Requirements)
3. Non-Functional Requirements
4. Data Model (High level entities)
5. API Endpoints (High level)

Return ONLY the content of the spec file. Do not use markdown code blocks.`,
			m.projectNameInput.Value(), m.pitchInput.Value(), transcript)

		resp, err := m.agent.Send(context.Background(), prompt)
		if err != nil {
			return specMsg{err: err}
		}

		// Clean up response if it's wrapped in code blocks
		spec := cleanCodeBlock(resp)
		return specMsg{spec: spec}
	}
}

func parseJSONList(input string, target interface{}) error {
	input = cleanCodeBlock(input)
	return json.Unmarshal([]byte(input), target)
}

func cleanCodeBlock(input string) string {
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "```") {
		lines := strings.Split(input, "\n")
		if len(lines) >= 2 {
			// Find end
			end := len(lines) - 1
			if strings.HasPrefix(lines[end], "```") {
				return strings.Join(lines[1:end], "\n")
			}
			return strings.Join(lines[1:], "\n")
		}
	}
	return input
}

// -- Update --

func (m draftModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m draftModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || (msg.String() == "q" && m.state == StateDraftDone) {
			return m, tea.Quit
		}

		switch m.state {
		case StateDraftProjectName:
			if msg.Type == tea.KeyEnter && m.projectNameInput.Value() != "" {
				m.state = StateDraftPitch
				m.pitchInput.Placeholder = "Describe your idea..."
				m.pitchInput.Focus()
				return m, nil // No command needed yet
			}

		case StateDraftPitch:
			if msg.String() == "ctrl+s" && m.pitchInput.Value() != "" {
				m.state = StateDraftThinkingQuestions
				return m, generateDraftQuestionsCmd(m)
			}

		case StateDraftAnsweringQuestions:
			if msg.String() == "ctrl+s" {
				m.answers = append(m.answers, m.answerInput.Value())
				m.answerInput.Reset()
				m.currQIdx++

				if m.currQIdx >= len(m.questions) {
					m.state = StateDraftThinkingSpec
					return m, generateDraftSpecCmd(m)
				}
				// Next question
				m.answerInput.Placeholder = "Answer here..."
				m.answerInput.Focus()
				return m, nil
			}

		case StateDraftReview:
			// After spec is generated, user sees it.
			if msg.Type == tea.KeyEnter {
				// Save file
				if err := os.WriteFile(m.outPath, []byte(m.finalSpec), 0644); err != nil {
					m.err = err
					return m, tea.Quit
				}
				m.state = StateDraftDone
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport = viewport.New(msg.Width, msg.Height-10)
		m.projectNameInput.Width = msg.Width - 4
		m.pitchInput.SetWidth(msg.Width - 4)
		m.answerInput.SetWidth(msg.Width - 4)

	case questionsMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.questions = msg.questions
		m.state = StateDraftAnsweringQuestions
		m.currQIdx = 0
		m.answerInput.Focus()

	case specMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.finalSpec = msg.spec
		m.state = StateDraftReview
		// Setup viewport for review
		rendered, _ := m.renderer.Render(m.finalSpec)
		m.viewport.SetContent(rendered)
	}

	// Forward events to components
	switch m.state {
	case StateDraftProjectName:
		m.projectNameInput, cmd = m.projectNameInput.Update(msg)
		cmds = append(cmds, cmd)
	case StateDraftPitch:
		m.pitchInput, cmd = m.pitchInput.Update(msg)
		cmds = append(cmds, cmd)
	case StateDraftAnsweringQuestions:
		m.answerInput, cmd = m.answerInput.Update(msg)
		cmds = append(cmds, cmd)
	case StateDraftReview:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// -- View --

func (m draftModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render("Architect Draft Wizard")

	var content string

	switch m.state {
	case StateDraftProjectName:
		content = fmt.Sprintf("What is the name of your project?\n\n%s", m.projectNameInput.View())

	case StateDraftPitch:
		content = fmt.Sprintf("Describe your project (Pitch):\n\n%s\n\n%s", m.pitchInput.View(), subtext("Ctrl+S to submit"))

	case StateDraftThinkingQuestions:
		content = "Thinking of clarifying questions..."

	case StateDraftAnsweringQuestions:
		if m.currQIdx < len(m.questions) {
			q := m.questions[m.currQIdx]
			qText, _ := m.renderer.Render(q.Question)
			content = fmt.Sprintf("Question %d/%d:\n%s\nContext: %s\n\n%s\n\n%s",
				m.currQIdx+1, len(m.questions), qText, q.Context, m.answerInput.View(), subtext("Ctrl+S to submit"))
		}

	case StateDraftThinkingSpec:
		content = "Generating specification..."

	case StateDraftReview:
		content = fmt.Sprintf("Here is the generated spec. Press Enter to save to '%s'.\n\n%s", m.outPath, m.viewport.View())

	case StateDraftDone:
		content = fmt.Sprintf("Done! Saved to %s.", m.outPath)
	}

	return fmt.Sprintf("%s\n\n%s", header, content)
}

func subtext(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(s)
}
