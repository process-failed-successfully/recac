package tui

import (
	"recac/internal/agent"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestSessionModel_Update(t *testing.T) {
	mockAg := agent.NewMockAgent()
	m := NewSessionModel(mockAg)

	// Test initial state
	assert.Empty(t, m.messages)
	assert.False(t, m.isLoading)

	// Test Enter with empty input -> No Op
	m.input.SetValue("")
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Avoid DeepEqual on complex structs, check state
	sessM := newM.(SessionModel)
	assert.Empty(t, sessM.messages)
	assert.Nil(t, cmd)

	// Test Enter with input -> Send Request
	m.input.SetValue("Hello")
	newM, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})

	sessM = newM.(SessionModel)
	assert.True(t, sessM.isLoading)
	assert.Equal(t, "", sessM.input.Value()) // Input cleared
	assert.Contains(t, sessM.history, "**User**: Hello")
	assert.NotNil(t, cmd)

	// Simulate streaming chunk
	chunk := "Hi there"
	ch := make(chan string, 1)
	ch <- "next"
	close(ch) // Close to prevent blocking if read multiple times

	// We need to pass the SAME channel in chunkMsg so waitForChunk can use it
	newM, cmd = sessM.Update(chunkMsg{chunk: chunk, ch: ch})
	sessM = newM.(SessionModel)

	// Chunk is in currentResponse now, not yet in history
	assert.Contains(t, sessM.currentResponse, chunk)
	assert.NotContains(t, sessM.history, chunk)
	assert.NotNil(t, cmd) // waitForChunk

	// Simulate done
	newM, cmd = sessM.Update(doneMsg{})
	sessM = newM.(SessionModel)
	assert.False(t, sessM.isLoading)
	assert.Contains(t, sessM.history, chunk) // Now it's in history
	assert.Empty(t, sessM.currentResponse)
	assert.Nil(t, cmd)
}

func TestSessionModel_Commands(t *testing.T) {
	m := NewSessionModel(nil)

	// /help
	m.input.SetValue("/help")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "Available commands")

	// /clear
	sessM.history = "Some history"
	sessM.input.SetValue("/clear")
	newM, _ = sessM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM = newM.(SessionModel)
	assert.Empty(t, sessM.history)

	// /quit
	sessM.input.SetValue("/quit")
	_, cmd := sessM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.NotNil(t, cmd)

	// Check if cmd is Quit
	// tea.Quit is a function. Comparison is hard.
	// Invoke it and check message type
	msg := cmd()
	_, ok := msg.(tea.QuitMsg)
	assert.True(t, ok, "Expected tea.QuitMsg")
}
