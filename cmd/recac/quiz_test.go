package main

import (
	"context"
	"recac/internal/agent"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestQuizCmd(t *testing.T) {
	// 1. Setup Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockAgentClient)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// 2. Mock Response
	// The quiz command expects JSON
	jsonResponse := `[
		{
			"question": "What is the meaning of life?",
			"options": ["42", "24", "0", "1"],
			"correct_answer_index": 0,
			"explanation": "Because Douglas Adams said so."
		}
	]`

	// Expect Send to be called
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonResponse, nil)

	// Since we cannot run the full command easily due to TUI, we test the parts we can.
	// We verify that the command exists and flags are correct.
	cmd := quizCmd
	assert.NotNil(t, cmd)

	limit, err := cmd.Flags().GetInt("questions")
	assert.NoError(t, err)
	assert.Equal(t, 5, limit)
}

func TestQuizModel_Update(t *testing.T) {
	questions := []QuizQuestion{
		{
			Question:      "Q1",
			Options:       []string{"A", "B"},
			CorrectAnswer: 0,
			Explanation:   "Exp1",
		},
		{
			Question:      "Q2",
			Options:       []string{"C", "D"},
			CorrectAnswer: 1,
			Explanation:   "Exp2",
		},
	}

	m := InitialQuizModel(questions)

	// 1. Initial State
	assert.Equal(t, 0, m.current)
	assert.Equal(t, 0, m.score)
	assert.Equal(t, 0, m.selectedOption)
	assert.False(t, m.showResult)
	assert.False(t, m.finished)

	// 2. Move Selection Down
	msg := tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	newM, _ := m.Update(msg)
	m = newM.(QuizModel)
	assert.Equal(t, 1, m.selectedOption)

	// 3. Select Wrong Answer (Option B, index 1) for Q1 (Correct is 0)
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	newM, _ = m.Update(msg)
	m = newM.(QuizModel)
	assert.True(t, m.showResult)
	assert.Equal(t, 0, m.score) // Wrong answer

	// 4. Next Question
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	newM, _ = m.Update(msg)
	m = newM.(QuizModel)
	assert.False(t, m.showResult)
	assert.Equal(t, 1, m.current)
	assert.Equal(t, 0, m.selectedOption) // Reset selection

	// 5. Select Correct Answer (Option D, index 1) for Q2 (Correct is 1)
	// Move Down first
	msg = tea.KeyMsg{Type: tea.KeyDown, Runes: []rune{}}
	newM, _ = m.Update(msg)
	m = newM.(QuizModel)
	assert.Equal(t, 1, m.selectedOption)

	// Press Enter
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	newM, _ = m.Update(msg)
	m = newM.(QuizModel)
	assert.True(t, m.showResult)
	assert.Equal(t, 1, m.score) // Correct answer!

	// 6. Finish
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	newM, _ = m.Update(msg)
	m = newM.(QuizModel)
	assert.True(t, m.finished)

	// 7. Quit
	msg = tea.KeyMsg{Type: tea.KeyEnter, Runes: []rune{}}
	_, cmd := m.Update(msg)
	assert.Equal(t, tea.Quit(), cmd())
}
