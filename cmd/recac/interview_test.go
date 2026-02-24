package main

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockInterviewAgent implementation for testing
type MockInterviewAgent struct {
	QuestionResponse   string
	EvaluationResponse string
	CallCount          int
}

func (m *MockInterviewAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CallCount++
	// Simple heuristic: if prompt contains "Evaluate", return evaluation, else question
	if strings.Contains(prompt, "Evaluate") {
		return m.EvaluationResponse, nil
	}
	return m.QuestionResponse, nil
}

func (m *MockInterviewAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestInterview_RepoContext(t *testing.T) {
	// Mock Context Generator
	origContextFunc := interviewContextFunc
	called := false
	interviewContextFunc = func(opts ContextOptions) (string, error) {
		called = true
		return "Mock Repo Context", nil
	}
	defer func() { interviewContextFunc = origContextFunc }()

	// Verify we can call the swapped function
	res, err := interviewContextFunc(ContextOptions{})
	assert.NoError(t, err)
	assert.Equal(t, "Mock Repo Context", res)
	assert.True(t, called)
}

func TestInterviewModel_Update_Flow(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &MockInterviewAgent{
		QuestionResponse: `{"question": "What is 2+2?", "context": "Math"}`,
		EvaluationResponse: `{"feedback": "Good job", "score": 10, "is_correct": true, "follow_up": "What is 4+4?"}`,
	}

	session := &InterviewSession{
		Topic:     "Math",
		Level:     "Junior",
		MaxRounds: 2,
		Current:   0,
		Score:     0,
		Agent:     mockAgent,
	}

	m := initialInterviewModel(session, "")

	// 1. Init should return a command to generate question
	cmd := m.Init()
	assert.NotNil(t, cmd)

	// Execute the command (simulate)
	msg := cmd()
	qMsg, ok := msg.(questionMsg)
	require.True(t, ok)
	assert.NoError(t, qMsg.err)
	assert.Equal(t, "What is 2+2?", qMsg.q.Question)

	// 2. Update model with Question
	newM, _ := m.Update(qMsg)
	m = newM.(interviewModel)
	assert.Equal(t, StateAnswer, m.state)
	assert.Equal(t, "What is 2+2?", m.currentQuestion.Question)

	// 3. User types answer
	m.textarea.SetValue("4")

	// 4. User submits (Ctrl+S)
	submitMsg := tea.KeyMsg{Type: tea.KeyCtrlS}
	newM, cmd = m.Update(submitMsg)
	m = newM.(interviewModel)

	assert.Equal(t, StateEvaluation, m.state)
	assert.NotNil(t, cmd) // This should be evaluate command

	// Execute Evaluate Command
	msg = cmd()
	eMsg, ok := msg.(evaluationMsg)
	require.True(t, ok)
	assert.NoError(t, eMsg.err)
	assert.Equal(t, 10, eMsg.eval.Score)

	// 5. Update model with Evaluation
	newM, _ = m.Update(eMsg)
	m = newM.(interviewModel)

	assert.Equal(t, StateEvaluation, m.state) // Remains in evaluation until user proceeds
	assert.Equal(t, 10, m.session.Score)

	// 6. User proceeds (Enter)
	proceedMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, cmd = m.Update(proceedMsg)
	m = newM.(interviewModel)

	// Should go to next round (StateLoading)
	assert.Equal(t, StateLoading, m.state)
	assert.Equal(t, 1, m.session.Current)
	assert.NotNil(t, cmd) // Next question

	// Execute next question command
	msg = cmd()
	qMsg, ok = msg.(questionMsg)
	require.True(t, ok)

	// Update with Question 2
	newM, _ = m.Update(qMsg)
	m = newM.(interviewModel)
	assert.Equal(t, StateAnswer, m.state)

	// Submit answer for Q2
	m.textarea.SetValue("4")
	newM, cmd = m.Update(submitMsg)
	m = newM.(interviewModel)

	// Execute Evaluate Q2
	msg = cmd()
	eMsg, ok = msg.(evaluationMsg)
	require.True(t, ok)

	newM, _ = m.Update(eMsg)
	m = newM.(interviewModel)
	assert.Equal(t, 20, m.session.Score) // 10 + 10

	// Proceed (Enter) -> Should finish
	newM, cmd = m.Update(proceedMsg)
	m = newM.(interviewModel)

	// Since Current was 1, increment to 2. MaxRounds is 2. So 2 >= 2 -> Finished.
	assert.Equal(t, StateFinished, m.state)

	// Quit
	quitMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	newM, cmd = m.Update(quitMsg)
	assert.NotNil(t, cmd)
}

func TestParseJSON(t *testing.T) {
	// Test basic JSON
	var q InterviewQuestion
	err := parseJSON(`{"question": "foo"}`, &q)
	assert.NoError(t, err)
	assert.Equal(t, "foo", q.Question)

	// Test Markdown JSON
	err = parseJSON("```json\n{\"question\": \"bar\"}\n```", &q)
	assert.NoError(t, err)
	assert.Equal(t, "bar", q.Question)

	// Test Markdown without lang
	err = parseJSON("```\n{\"question\": \"baz\"}\n```", &q)
	assert.NoError(t, err)
	assert.Equal(t, "baz", q.Question)
}
