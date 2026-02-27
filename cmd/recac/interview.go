package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// -- Dependency Injection --
var (
	interviewAgentFactory = agent.NewAgent
	interviewContextFunc  = GenerateCodebaseContext
)

// -- Flags --
var (
	interviewTopic  string
	interviewLevel  string
	interviewRounds int
)

// -- Command Definition --
var interviewCmd = &cobra.Command{
	Use:   "interview",
	Short: "Start a mock technical interview with AI",
	Long: `Starts an interactive mock interview session where the AI acts as the interviewer.
You can choose a specific topic, difficulty level, or have it interview you about this codebase.`,
	RunE: runInterview,
}

func init() {
	interviewCmd.Flags().StringVarP(&interviewTopic, "topic", "t", "General", "Interview topic (e.g., 'Go', 'System Design', 'Repository')")
	interviewCmd.Flags().StringVarP(&interviewLevel, "level", "l", "Senior", "Difficulty level")
	interviewCmd.Flags().IntVarP(&interviewRounds, "rounds", "r", 3, "Number of questions")

	if rootCmd != nil {
		rootCmd.AddCommand(interviewCmd)
	}
}

// -- Data Structures --

type InterviewQuestion struct {
	Question string `json:"question"`
	Context  string `json:"context,omitempty"` // Optional hint or context
}

type InterviewEvaluation struct {
	Feedback  string `json:"feedback"`
	Score     int    `json:"score"` // 1-10
	FollowUp  string `json:"follow_up,omitempty"`
	IsCorrect bool   `json:"is_correct"`
}

type InterviewSession struct {
	Topic     string
	Level     string
	MaxRounds int
	Current   int
	Score     int
	History   []string // Store conversation history for context
	Agent     agent.Agent
}

// Factory functions
var startInterviewTUIFunc = func(m tea.Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// -- Main Entry Point --

func runInterview(cmd *cobra.Command, args []string) error {
	// 1. Setup Context (if Repository)
	var repoContext string
	if strings.ToLower(interviewTopic) == "repository" || strings.ToLower(interviewTopic) == "repo" {
		fmt.Println("Analyzing repository for interview context...")
		var err error
		repoContext, err = interviewContextFunc(ContextOptions{
			MaxSize: 100 * 1024, // 100KB limit
			Tree:    true,
		})
		if err != nil {
			return fmt.Errorf("failed to generate repo context: %w", err)
		}
	}

	// 2. Initialize Agent
	provider := viper.GetString("agent_provider")
	if provider == "" {
		provider = os.Getenv("RECAC_AGENT_PROVIDER")
	}
	if provider == "" {
		provider = "mock"
	}

	// Use explicit config/env for model if set, otherwise default
	model := viper.GetString("agent_model")
	apiKey := viper.GetString("api_key")

	ag, err := interviewAgentFactory(provider, apiKey, model, ".", "recac-interview")
	if err != nil {
		return fmt.Errorf("failed to init agent: %w", err)
	}

	session := &InterviewSession{
		Topic:     interviewTopic,
		Level:     interviewLevel,
		MaxRounds: interviewRounds,
		Current:   0,
		Score:     0,
		Agent:     ag,
	}

	// 3. Start TUI
	// Initial question generation happens in Init()
	modelTUI := initialInterviewModel(session, repoContext)

	if err := startInterviewTUIFunc(modelTUI); err != nil {
		return fmt.Errorf("error running interview: %w", err)
	}

	return nil
}

// -- TUI Model --

type InterviewState int

const (
	StateLoading InterviewState = iota
	StateQuestion
	StateAnswer
	StateEvaluation
	StateFinished
)

type interviewModel struct {
	session     *InterviewSession
	repoContext string
	state       InterviewState

	currentQuestion *InterviewQuestion
	lastEvaluation  *InterviewEvaluation

	textarea    textarea.Model
	viewport    viewport.Model
	renderer    *glamour.TermRenderer
	err         error

	width, height int
}

func initialInterviewModel(session *InterviewSession, repoContext string) interviewModel {
	ta := textarea.New()
	ta.Placeholder = "Type your answer here..."
	ta.Focus()
	ta.CharLimit = 2000
	ta.SetHeight(5)
	ta.ShowLineNumbers = false

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return interviewModel{
		session:     session,
		repoContext: repoContext,
		state:       StateLoading,
		textarea:    ta,
		renderer:    r,
	}
}

// -- Messages --

type questionMsg struct {
	q   *InterviewQuestion
	err error
}

type evaluationMsg struct {
	eval *InterviewEvaluation
	err  error
}

// -- Commands --

func generateQuestionCmd(m interviewModel) tea.Cmd {
	return func() tea.Msg {
		prompt := fmt.Sprintf(`You are a technical interviewer conducting a %s level interview on the topic: %s.
Current Round: %d of %d.
Generate a challenging technical question.
Return ONLY a JSON object with keys: "question" (string) and "context" (string, optional).

Interview History:
%s

`, m.session.Level, m.session.Topic, m.session.Current+1, m.session.MaxRounds, strings.Join(m.session.History, "\n"))

		if m.repoContext != "" {
			prompt += fmt.Sprintf("\nContext from Repository:\n%s\n", m.repoContext)
		}

		resp, err := m.session.Agent.Send(context.Background(), prompt)
		if err != nil {
			return questionMsg{err: err}
		}

		// Parse JSON
		var q InterviewQuestion
		if err := parseJSON(resp, &q); err != nil {
			// Fallback if not valid JSON
			q = InterviewQuestion{Question: resp}
		}

		return questionMsg{q: &q}
	}
}

func evaluateAnswerCmd(m interviewModel, answer string) tea.Cmd {
	return func() tea.Msg {
		prompt := fmt.Sprintf(`Evaluate the candidate's answer.
Question: %s
Candidate Answer: %s

Return ONLY a JSON object with keys:
- "feedback" (string): Detailed feedback.
- "score" (int): 1-10.
- "is_correct" (bool): Whether the answer is acceptable.
- "follow_up" (string): A short follow-up comment.
`, m.currentQuestion.Question, answer)

		resp, err := m.session.Agent.Send(context.Background(), prompt)
		if err != nil {
			return evaluationMsg{err: err}
		}

		var eval InterviewEvaluation
		if err := parseJSON(resp, &eval); err != nil {
			return evaluationMsg{err: fmt.Errorf("failed to parse evaluation: %w", err)}
		}

		return evaluationMsg{eval: &eval}
	}
}

func parseJSON(input string, target interface{}) error {
	// Clean markdown code blocks
	input = strings.TrimSpace(input)
	if strings.HasPrefix(input, "```") {
		lines := strings.Split(input, "\n")
		if len(lines) > 2 {
			input = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}
	return json.Unmarshal([]byte(input), target)
}

// -- Update Loop --

func (m interviewModel) Init() tea.Cmd {
	return generateQuestionCmd(m)
}

func (m interviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		}

		// Handle specific states
		switch m.state {
		case StateAnswer:
			if msg.String() == "enter" && !msg.Alt { // Enter to submit, Alt+Enter for new line?
				// Actually textarea handles enter as newline by default.
				// Let's use Ctrl+S or just check if they are done?
				// Common pattern: Enter adds newline, Ctrl+S submits.
				// Or use a distinct key.
			}
			// For this implementation, let's say Ctrl+S submits.
			if msg.String() == "ctrl+s" {
				if m.textarea.Value() == "" {
					return m, nil
				}
				m.state = StateEvaluation
				m.session.History = append(m.session.History, fmt.Sprintf("Q: %s\nA: %s", m.currentQuestion.Question, m.textarea.Value()))
				cmds = append(cmds, evaluateAnswerCmd(m, m.textarea.Value()))
				m.textarea.Reset()
				return m, tea.Batch(cmds...)
			}

		case StateEvaluation:
			if msg.String() == "enter" {
				m.session.Current++
				if m.session.Current >= m.session.MaxRounds {
					m.state = StateFinished
				} else {
					m.state = StateLoading
					cmds = append(cmds, generateQuestionCmd(m))
				}
			}

		case StateFinished:
			if msg.String() == "q" {
				return m, tea.Quit
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.textarea.SetWidth(msg.Width - 4)
		m.viewport = viewport.New(msg.Width, msg.Height-10) // Reserve space for header/footer

	case questionMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.currentQuestion = msg.q
		m.state = StateAnswer
		m.textarea.Focus()

	case evaluationMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.lastEvaluation = msg.eval
		m.session.Score += msg.eval.Score
		m.session.History = append(m.session.History, fmt.Sprintf("Feedback: %s", msg.eval.Feedback))

	}

	if m.state == StateAnswer {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// -- View --

func (m interviewModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	header := lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).Render(
		fmt.Sprintf("Interview: %s (%s) | Round %d/%d", m.session.Topic, m.session.Level, m.session.Current+1, m.session.MaxRounds))

	var content string

	switch m.state {
	case StateLoading:
		content = "Thinking..."

	case StateAnswer:
		q := m.currentQuestion.Question
		if m.currentQuestion.Context != "" {
			q += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.currentQuestion.Context)
		}
		renderedQ, _ := m.renderer.Render(q)
		content = fmt.Sprintf("%s\n\n%s\n\n%s", renderedQ, m.textarea.View(), lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("Ctrl+S to submit"))

	case StateEvaluation:
		eval := m.lastEvaluation
		scoreColor := "196" // Red
		if eval.Score >= 7 {
			scoreColor = "46" // Green
		} else if eval.Score >= 5 {
			scoreColor = "220" // Yellow
		}

		scoreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(scoreColor)).Bold(true)

		fb, _ := m.renderer.Render(eval.Feedback)

		content = fmt.Sprintf("Score: %s/10\n\n%s\n\nPress Enter for next question...", scoreStyle.Render(fmt.Sprintf("%d", eval.Score)), fb)

	case StateFinished:
		totalScore := m.session.Score
		maxScore := m.session.MaxRounds * 10
		percentage := float64(totalScore) / float64(maxScore) * 100

		msg := "Interview Complete!"
		if percentage >= 80 {
			msg += " You're hired!"
		} else {
			msg += " Keep practicing."
		}

		content = fmt.Sprintf("%s\n\nTotal Score: %d/%d (%.1f%%)\n\nPress q to quit.", msg, totalScore, maxScore, percentage)
	}

	return fmt.Sprintf("%s\n\n%s", header, content)
}
