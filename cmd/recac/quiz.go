package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	quizTopic     string
	quizQuestions int
)

var quizCmd = &cobra.Command{
	Use:   "quiz",
	Short: "Test your knowledge of the codebase",
	Long: `Generates interactive quiz questions about the codebase using AI.
This helps onboard new developers or refresh your memory about specific components.`,
	RunE: runQuiz,
}

func init() {
	rootCmd.AddCommand(quizCmd)
	quizCmd.Flags().StringVarP(&quizTopic, "topic", "t", "general", "Topic to focus on (e.g., 'auth', 'database', 'api')")
	quizCmd.Flags().IntVarP(&quizQuestions, "questions", "n", 5, "Number of questions to generate")
}

// Data Structures

type QuizQuestion struct {
	Question     string   `json:"question"`
	Options      []string `json:"options"`
	CorrectIndex int      `json:"correct_index"`
	Explanation  string   `json:"explanation"`
}

type QuizModel struct {
	questions []QuizQuestion
	index     int
	score     int
	width     int
	height    int

	// State
	selected int // Index of selected option (0-3)
	answered bool
	feedback string
	done     bool

	// Components
	progress progress.Model
}

// Styles
var (
	questionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	optionStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Foreground(lipgloss.Color("252"))

	selectedOptionStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Foreground(lipgloss.Color("205")).
				Bold(true)

	correctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	incorrectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	explanationStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244")).
				MarginTop(1)

	scoreStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true).
			MarginTop(2)
)

func runQuiz(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Generate Context (Limited)
	// We use the existing GenerateCodebaseContext but restrict size to save tokens/time
	opts := ContextOptions{
		Roots:     []string{"."},
		MaxSize:   50 * 1024, // 50KB limit per file
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "📚 Analyzing codebase context...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prompt Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-quiz")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a senior engineer onboarding a new team member.
Generate %d multiple-choice questions about the provided codebase to test their understanding.
Focus on: %s

The questions should cover:
- Architecture and design patterns used
- Key data structures and their purpose
- The flow of data between components
- Specific implementation details of important functions

Return the result as a raw JSON list of objects with this structure:
[
  {
    "question": "What is the primary responsibility of the 'User' struct?",
    "options": ["Auth", "Data storage", "API response", "Logging"],
    "correct_index": 1,
    "explanation": "The User struct maps directly to the users table in the database."
  }
]

Do not include markdown formatting (like `+"```json"+`). Just raw JSON.

CODEBASE CONTEXT:
%s`, quizQuestions, quizTopic, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating quiz questions (this may take a moment)...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 3. Parse Questions
	jsonStr := utils.CleanJSONBlock(resp)
	var questions []QuizQuestion
	if err := json.Unmarshal([]byte(jsonStr), &questions); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to parse JSON: %v\nRaw: %s\n", err, resp)
		return fmt.Errorf("failed to parse agent response")
	}

	if len(questions) == 0 {
		return fmt.Errorf("no questions generated")
	}

	// 4. Start TUI
	p := tea.NewProgram(initialModel(questions))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run quiz: %w", err)
	}

	return nil
}

func initialModel(questions []QuizQuestion) QuizModel {
	return QuizModel{
		questions: questions,
		progress:  progress.New(progress.WithDefaultGradient()),
		index:     0,
		score:     0,
		selected:  0,
		answered:  false,
	}
}

func (m QuizModel) Init() tea.Cmd {
	return nil
}

func (m QuizModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			if !m.answered && m.selected > 0 {
				m.selected--
			}

		case "down", "j":
			if !m.answered && m.selected < len(m.questions[m.index].Options)-1 {
				m.selected++
			}

		case "enter", "space":
			if m.done {
				return m, tea.Quit
			}

			if !m.answered {
				// Answer the question
				m.answered = true
				correct := m.questions[m.index].CorrectIndex
				if m.selected == correct {
					m.score++
					m.feedback = "✅ Correct!"
				} else {
					m.feedback = fmt.Sprintf("❌ Incorrect. The correct answer was: %s", m.questions[m.index].Options[correct])
				}
			} else {
				// Next question
				if m.index < len(m.questions)-1 {
					m.index++
					m.selected = 0
					m.answered = false
					m.feedback = ""
				} else {
					m.done = true
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 4
	}

	return m, nil
}

func (m QuizModel) View() string {
	if m.done {
		return m.viewSummary()
	}
	return m.viewQuestion()
}

func (m QuizModel) viewQuestion() string {
	q := m.questions[m.index]

	var sb strings.Builder

	// Header / Progress
	prog := float64(m.index) / float64(len(m.questions))
	sb.WriteString(m.progress.ViewAs(prog) + "\n\n")

	// Question
	sb.WriteString(questionStyle.Render(fmt.Sprintf("Question %d: %s", m.index+1, q.Question)) + "\n\n")

	// Options
	for i, opt := range q.Options {
		cursor := "  "
		style := optionStyle

		if m.selected == i {
			cursor = "> "
			style = selectedOptionStyle
		}

		if m.answered {
			if i == q.CorrectIndex {
				style = correctStyle
				if m.selected == i {
					cursor = "✓ "
				} else {
					cursor = "  "
				}
			} else if i == m.selected && i != q.CorrectIndex {
				style = incorrectStyle
				cursor = "✗ "
			}
		}

		sb.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(opt)))
	}

	// Feedback / Explanation
	if m.answered {
		sb.WriteString("\n" + m.feedback + "\n")
		sb.WriteString(explanationStyle.Render(q.Explanation) + "\n\n")
		sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Enter to continue..."))
	} else {
		sb.WriteString("\n" + lipgloss.NewStyle().Faint(true).Render("Use arrow keys to select, Enter to confirm"))
	}

	return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
}

func (m QuizModel) viewSummary() string {
	var sb strings.Builder

	scorePct := float64(m.score) / float64(len(m.questions)) * 100
	grade := ""
	if scorePct >= 90 {
		grade = "🏆 Excellent!"
	} else if scorePct >= 70 {
		grade = "👏 Good job!"
	} else {
		grade = "📚 Keep learning!"
	}

	sb.WriteString(questionStyle.Render("Quiz Complete!") + "\n\n")
	sb.WriteString(scoreStyle.Render(fmt.Sprintf("Score: %d / %d (%.0f%%)", m.score, len(m.questions), scorePct)) + "\n")
	sb.WriteString(grade + "\n\n")

	sb.WriteString(lipgloss.NewStyle().Faint(true).Render("Press q to exit"))

	return lipgloss.NewStyle().Margin(1, 2).Render(sb.String())
}
