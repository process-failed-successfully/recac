package main

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockDraftAgent struct {
	QuestionsResponse string
	SpecResponse      string
	ReceivedPrompts   []string
}

func (m *MockDraftAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.ReceivedPrompts = append(m.ReceivedPrompts, prompt)
	if strings.Contains(prompt, "Generate 3 clarifying questions") {
		return m.QuestionsResponse, nil
	}
	if strings.Contains(prompt, "Create a comprehensive application specification") {
		return m.SpecResponse, nil
	}
	return "", nil
}

func (m *MockDraftAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestArchitectDraft_Flow(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &MockDraftAgent{
		QuestionsResponse: `[{"question": "Q1", "context": "C1"}, {"question": "Q2"}]`,
		SpecResponse:      "This is the final spec.",
	}

	// Create temp output file
	tmpFile, err := os.CreateTemp("", "app_spec_test_*.txt")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Init Model
	m := initialDraftModel(mockAgent, tmpPath)

	// 1. Initial State: Project Name
	assert.Equal(t, StateDraftProjectName, m.state)

	// User types project name and hits Enter
	m.projectNameInput.SetValue("MyProject")
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}

	newM, _ := m.Update(enterMsg)
	m = newM.(draftModel)
	assert.Equal(t, StateDraftPitch, m.state)

	// 2. Pitch State
	// User types pitch and hits Ctrl+S
	m.pitchInput.SetValue("MyPitch")
	ctrlSMsg := tea.KeyMsg{Type: tea.KeyCtrlS}

	newM, cmd := m.Update(ctrlSMsg)
	m = newM.(draftModel)
	assert.Equal(t, StateDraftThinkingQuestions, m.state)

	// Execute command (Generate Questions)
	assert.NotNil(t, cmd)
	msg := cmd()
	qMsg, ok := msg.(questionsMsg)
	require.True(t, ok)
	assert.NoError(t, qMsg.err)
	assert.Len(t, qMsg.questions, 2)

	// Update with questions
	newM, _ = m.Update(qMsg)
	m = newM.(draftModel)
	assert.Equal(t, StateDraftAnsweringQuestions, m.state)
	assert.Equal(t, 0, m.currQIdx)

	// 3. Answer Questions
	// Q1
	m.answerInput.SetValue("A1")
	newM, _ = m.Update(ctrlSMsg) // Submit A1
	m = newM.(draftModel)
	assert.Equal(t, 1, m.currQIdx)
	assert.Len(t, m.answers, 1)

	// Q2
	m.answerInput.SetValue("A2")
	newM, cmd = m.Update(ctrlSMsg) // Submit A2
	m = newM.(draftModel)

	// Should transition to generating spec
	assert.Equal(t, StateDraftThinkingSpec, m.state)
	assert.NotNil(t, cmd)

	// Execute command (Generate Spec)
	msg = cmd()
	sMsg, ok := msg.(specMsg)
	require.True(t, ok)
	assert.NoError(t, sMsg.err)
	assert.Equal(t, "This is the final spec.", sMsg.spec)

	// Update with spec
	newM, _ = m.Update(sMsg)
	m = newM.(draftModel)
	assert.Equal(t, StateDraftReview, m.state)
	assert.Equal(t, "This is the final spec.", m.finalSpec)

	// 4. Review and Save
	newM, cmd = m.Update(enterMsg)
	m = newM.(draftModel)
	assert.Equal(t, StateDraftDone, m.state)

	// Check file content
	content, err := os.ReadFile(tmpPath)
	assert.NoError(t, err)
	assert.Equal(t, "This is the final spec.", string(content))
}
