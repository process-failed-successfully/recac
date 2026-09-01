package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"recac/internal/utils"
)

var (
	quizLimit int
	quizFocus string
)

var quizCmd = &cobra.Command{
	Use:   "quiz",
	Short: "Take an interactive quiz about the codebase",
	Long: `Generates a multiple-choice quiz based on the current codebase to help you learn the architecture and implementation details.
It uses AI to analyze the code and generate relevant questions.`,
	RunE: runQuiz,
}

func init() {
	rootCmd.AddCommand(quizCmd)
	quizCmd.Flags().IntVarP(&quizLimit, "questions", "n", 5, "Number of questions to generate")
	quizCmd.Flags().StringVarP(&quizFocus, "focus", "f", ".", "Focus on a specific path")
}

type QuizQuestion struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	CorrectAnswer int      `json:"correct_answer_index"` // 0-based index
	Explanation   string   `json:"explanation"`
}

type QuizModel struct {
	questions      []QuizQuestion
	current        int
	score          int
	selectedOption int
	showResult     bool
	finished       bool
	width          int
	height         int
	err            error
}

func runQuiz(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	// 1. Generate Context
	opts := ContextOptions{
		Roots:     []string{quizFocus},
		MaxSize:   50 * 1024, // 50KB limit per file to save tokens
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing codebase...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-quiz")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a technical interviewer.
Generate %d multiple-choice questions about the following codebase.
Focus on architecture, key functions, and specific implementation details found in the context.
Make the questions challenging but fair.

Return the result as a raw JSON list of objects with the following structure:
[
  {
    "question": "The text of the question?",
    "options": ["Option A", "Option B", "Option C", "Option D"],
    "correct_answer_index": 0,
    "explanation": "Why this is the correct answer."
  }
]

Do not wrap the JSON in markdown code blocks. Just return the raw JSON string.

CODEBASE CONTEXT:
%s`, quizLimit, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating quiz questions (this may take a moment)...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// 3. Parse Questions
	jsonStr := utils.CleanJSONBlock(resp)
	var questions []QuizQuestion
	if err := json.Unmarshal([]byte(jsonStr), &questions); err != nil {
		return fmt.Errorf("failed to parse questions: %w\nResponse: %s", err, resp)
	}

	if len(questions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No questions generated.")
		return nil
	}

	// Shuffle questions? Maybe later. For now, keep order or random.
	// 4. Run TUI
	m := InitialQuizModel(questions)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running quiz: %w", err)
	}

	return nil
}

// TUI Implementation

var (
	questionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			Padding(1, 0)

	optionStyle = lipgloss.NewStyle().
			Padding(0, 2)

	selectedOptionStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Foreground(lipgloss.Color("229")).
				Background(lipgloss.Color("57"))

	correctStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)

	incorrectStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Bold(true)

	scoreStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginTop(1)
)

func InitialQuizModel(questions []QuizQuestion) QuizModel {
	return QuizModel{
		questions: questions,
		current:   0,
		score:     0,
	}
}

func (m QuizModel) Init() tea.Cmd {
	return nil
}

func (m QuizModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

		if m.finished {
			if msg.String() == "enter" {
				return m, tea.Quit
			}
			return m, nil
		}

		if m.showResult {
			if msg.String() == "enter" {
				m.showResult = false
				m.current++
				m.selectedOption = 0
				if m.current >= len(m.questions) {
					m.finished = true
				}
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "up", "k":
			if m.selectedOption > 0 {
				m.selectedOption--
			}
		case "down", "j":
			if m.selectedOption < len(m.questions[m.current].Options)-1 {
				m.selectedOption++
			}
		case "enter":
			m.showResult = true
			if m.selectedOption == m.questions[m.current].CorrectAnswer {
				m.score++
			}
		}
	}
	return m, nil
}

func (m QuizModel) View() string {
	if m.finished {
		return fmt.Sprintf("\n\n🎉 Quiz Finished!\n\nYour Score: %d/%d\n\nPress Enter to exit.\n", m.score, len(m.questions))
	}

	if m.current >= len(m.questions) {
		return "Done."
	}

	q := m.questions[m.current]
	var s strings.Builder

	s.WriteString(fmt.Sprintf("Question %d/%d\n", m.current+1, len(m.questions)))
	s.WriteString(questionStyle.Render(q.Question) + "\n\n")

	for i, opt := range q.Options {
		prefix := "  "
		style := optionStyle

		if m.showResult {
			if i == q.CorrectAnswer {
				prefix = "✅ "
				style = correctStyle
			} else if i == m.selectedOption && i != q.CorrectAnswer {
				prefix = "❌ "
				style = incorrectStyle
			} else {
				prefix = "   "
			}
		} else {
			if i == m.selectedOption {
				prefix = "> "
				style = selectedOptionStyle
			}
		}

		s.WriteString(fmt.Sprintf("%s%s\n", prefix, style.Render(opt)))
	}

	if m.showResult {
		s.WriteString("\n")
		s.WriteString(q.Explanation + "\n\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("Press Enter for next question..."))
	} else {
		s.WriteString("\n")
		s.WriteString(lipgloss.NewStyle().Faint(true).Render("Use ↑/↓ to select, Enter to confirm"))
	}

	s.WriteString(scoreStyle.Render(fmt.Sprintf("\nCurrent Score: %d", m.score)))

	return s.String()
}
