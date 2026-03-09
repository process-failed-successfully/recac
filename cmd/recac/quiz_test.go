package main

import (
	"context"
	"fmt"
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

	// Test Unready
	assert.Equal(t, "Initializing...", m.View())

	// Init viewport
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(quizModel)

	// Test Question View
	view := m.View()
	assert.Contains(t, view, "Question 1/1")
	assert.Contains(t, view, "Q1")
	assert.Contains(t, view, "A")
	assert.Contains(t, view, "B")

	// Test Selection
	m.selectedOption = 1
	view = m.View()
	assert.Contains(t, view, ">") // A bit brittle, but ensures rendering changes

	// Test Result View Correct
	m.selectedOption = 0
	m.showResult = true
	view = m.View()
	assert.Contains(t, view, "Explanation: Exp 1")
	assert.Contains(t, view, "✓")

	// Test Result View Incorrect
	m.selectedOption = 1
	m.showResult = true
	view = m.View()
	assert.Contains(t, view, "✗")

	// Test Finished View
	m.finished = true
	m.score = 1
	view = m.View()
	assert.Contains(t, view, "Quiz Complete!")
	assert.Contains(t, view, "Your Score: 1/1")
	assert.Contains(t, view, "Perfect score!")

	m.score = 0
	view = m.View()
	assert.Contains(t, view, "Time to read the docs a bit more.")
}

func TestQuizModel_Init(t *testing.T) {
	m := quizModel{}
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestRunQuiz_NoQuestions(t *testing.T) {
	// Mock context generator
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "Mock Context", nil
	}
	defer func() { generateContextFunc = origContextFunc }()

	// Mock factory to return empty list
	origFactory := quizAgentFactory
	defer func() { quizAgentFactory = origFactory }()

	quizAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return &MockQuizAgent{Response: `[]`}, nil
	}

	cmd := quizCmd
	err := runQuiz(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no questions generated")
}

func TestRunQuiz_ContextError(t *testing.T) {
	// Mock context generator
	origContextFunc := generateContextFunc
	generateContextFunc = func(opts ContextOptions) (string, error) {
		return "", fmt.Errorf("context error")
	}
	defer func() { generateContextFunc = origContextFunc }()

	cmd := quizCmd
	err := runQuiz(cmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate quiz")
}
