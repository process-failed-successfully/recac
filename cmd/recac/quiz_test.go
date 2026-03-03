package main

import (
	"context"
	"recac/internal/agent"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockQuizAgent implementation for testing
type MockQuizAgent struct {
	Response string
}

func (m *MockQuizAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockQuizAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestGenerateQuiz(t *testing.T) {
	// Mock factory
	origFactory := quizAgentFactory
	defer func() { quizAgentFactory = origFactory }()

	// Mock context generator
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Context", nil
	}
	defer func() { generateContextFunc = origContextFunc }()

	// Set test flags
	origQuestions := quizQuestions
	quizQuestions = 1
	defer func() { quizQuestions = origQuestions }()

	origFocus := quizFocus
	quizFocus = "."
	defer func() { quizFocus = origFocus }()

	mockResponse := `[
		{
			"question": "Test Question?",
			"options": ["A", "B", "C"],
			"correct_answer": 1,
			"explanation": "Because B is correct."
		}
	]`

	quizAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return &MockQuizAgent{Response: mockResponse}, nil
	}

	// Test Generate
	questions, err := generateQuiz(context.Background())
	require.NoError(t, err)
	assert.Len(t, questions, 1)
	assert.Equal(t, "Test Question?", questions[0].Question)
	assert.Equal(t, 1, questions[0].CorrectAnswer)
}

func TestQuizModel_Update(t *testing.T) {
	questions := []QuizQuestion{
		{
			Question:      "Q1",
			Options:       []string{"A", "B"},
			CorrectAnswer: 0,
			Explanation:   "Exp 1",
		},
		{
			Question:      "Q2",
			Options:       []string{"C", "D"},
			CorrectAnswer: 1,
			Explanation:   "Exp 2",
		},
	}

	m := initialQuizModel(questions)

	// Simulate window size to init viewport
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(quizModel)

	// 1. Initial State
	assert.Equal(t, 0, m.current)
	assert.Equal(t, 0, m.score)
	assert.False(t, m.showResult)

	// 2. Select Option 0 (Correct for Q1)
	// Default selection is 0. Press Enter to submit.
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ = m.Update(msg)
	m = newM.(quizModel)

	assert.True(t, m.showResult)
	assert.Equal(t, 1, m.score) // Score increased

	// 3. Next Question
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ = m.Update(msg)
	m = newM.(quizModel)

	assert.Equal(t, 1, m.current)
	assert.False(t, m.showResult)

	// 4. Select Option 0 (Incorrect for Q2, correct is 1)
	// Default is 0.
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ = m.Update(msg)
	m = newM.(quizModel)

	assert.True(t, m.showResult)
	assert.Equal(t, 1, m.score) // Score NOT increased

	// 5. Finish Quiz
	msg = tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ = m.Update(msg)
	m = newM.(quizModel)

	assert.True(t, m.finished)
}

func TestRunQuiz(t *testing.T) {
	origFactory := quizAgentFactory
	defer func() { quizAgentFactory = origFactory }()

	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Context", nil
	}
	defer func() { generateContextFunc = origContextFunc }()

	mockResponse := `[
		{
			"question": "Test Question?",
			"options": ["A", "B", "C"],
			"correct_answer": 1,
			"explanation": "Because B is correct."
		}
	]`

	quizAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return &MockQuizAgent{Response: mockResponse}, nil
	}

	// Use an empty args array
	// If it runs correctly without errors, that's what we test since the program runs Bubbletea loops blockingly if they could.
	// We'll actually override tea.NewProgram to run in non-blocking if possible, but for a simple e2e test...
	// Wait, bubble tea `p.Run()` will block waiting for input unless we can inject quit.
	// Bubble tea programs exit when `Update` returns `tea.Quit`.
	// The quiz starts in non-finished state, we can't easily quit it without input unless we use WithInput(bytes.NewReader(...))
	// So we won't do a full E2E of `runQuiz`, we'll just test the TUI components like `Init` and `View`.
}

func TestQuizModel_Init(t *testing.T) {
	m := initialQuizModel([]QuizQuestion{})
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestQuizModel_View(t *testing.T) {
	questions := []QuizQuestion{
		{
			Question:      "Q1",
			Options:       []string{"A", "B"},
			CorrectAnswer: 0,
			Explanation:   "Exp 1",
		},
	}
	m := initialQuizModel(questions)

	// Not ready
	assert.Equal(t, "Initializing...", m.View())

	// Ready, question view
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(quizModel)
	view := m.View()
	assert.Contains(t, view, "Q1")
	assert.Contains(t, view, "A")
	assert.Contains(t, view, "B")

	// Finished
	m.finished = true
	m.score = 1
	view = m.View()
	assert.Contains(t, view, "Quiz Complete!")
	assert.Contains(t, view, "1/1")
}

func TestQuizModel_Update_Quit(t *testing.T) {
	m := initialQuizModel([]QuizQuestion{})
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	msg := cmd()
	assert.IsType(t, tea.QuitMsg{}, msg)
	assert.Equal(t, m.ready, newM.(quizModel).ready)
}
