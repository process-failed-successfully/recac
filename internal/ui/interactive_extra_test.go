package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInteractiveModel_Update_Tab_NoCompletion_Extra(t *testing.T) {
	m := NewInteractiveModel(nil, "mock", "mock-model")
	m.textarea.SetValue("unknown")

	msg := tea.KeyMsg{Type: tea.KeyTab}
	newM, _ := m.Update(msg)
	finalM := newM.(InteractiveModel)

	// Should not change if no match
	assert.Equal(t, "unknown", finalM.textarea.Value())
}

func TestInteractiveModel_SetMode_Extra(t *testing.T) {
	m := NewInteractiveModel(nil, "mock", "mock-model")

	m.setMode(ModeAgentSelect)
	assert.Equal(t, ModeAgentSelect, m.mode)
	assert.True(t, m.showList)
	assert.Contains(t, m.list.Title, "Select Agent")

	m.setMode(ModePersonaSelect)
	assert.Equal(t, ModePersonaSelect, m.mode)
	assert.True(t, m.showList)
	assert.Contains(t, m.list.Title, "Select Persona")

	m.setMode(ModeCmd)
	assert.Equal(t, ModeCmd, m.mode)
	assert.True(t, m.showList)
}

func TestInteractiveModel_RenderSingleMessage_Extra(t *testing.T) {
	m := NewInteractiveModel(nil, "mock", "mock-model")
	m.width = 80
	m.viewport.Width = 80 // Set width for word wrap

	msg := ChatMessage{Role: RoleUser, Content: "hello"}
	out := m.renderSingleMessage(msg)
	assert.Contains(t, out, "You")
	assert.Contains(t, out, "hello")

	msgAgent := ChatMessage{Role: RoleBot, Content: "hi"}
	outAgent := m.renderSingleMessage(msgAgent)
	assert.Contains(t, outAgent, "Recac")
	assert.Contains(t, outAgent, "hi")
}

func TestInteractiveModel_Update_Keys_Extra(t *testing.T) {
	m := NewInteractiveModel(nil, "mock", "mock-model")

	// Test Back/Esc
	m.setMode(ModeAgentSelect)
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newM, _ := m.Update(msg)
	finalM := newM.(InteractiveModel)
	assert.Equal(t, ModeChat, finalM.mode)

	// Test Quit/Ctrl+C
	msgQuit := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msgQuit)
	assert.Equal(t, tea.Quit(), cmd())
}

func TestInteractiveModel_Update_Enter_Extra(t *testing.T) {
	m := NewInteractiveModel(nil, "mock", "mock-model")

	// Test Empty Enter (Should be ignored now)
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ := m.Update(msg)
	finalM := newM.(InteractiveModel)
	// Welcome message only
	assert.Len(t, finalM.messages, 1)

	// Test Send Message
	m.textarea.SetValue("Hello Agent")
	// Note: Update modifies the textarea state but we passed 'm' which has "Hello Agent"
	// But `m.textarea.Update` inside `Update` might process KeyEnter and modify value to have newline?
	// But `v = m.textarea.Value()` is called AFTER update.
	// If `InsertNewline` is true, `v` will be "Hello Agent\n".
	// `TrimSpace` will handle it.

	newM, cmd := m.Update(msg)
	finalM = newM.(InteractiveModel)

	// Should add user message AND system message
	// 1. Welcome
	// 2. User Message ("Hello Agent\n")
	// 3. System Message ("Processing...")
	assert.Len(t, finalM.messages, 3)
	assert.Contains(t, finalM.messages[1].Content, "Hello Agent")
	assert.Contains(t, finalM.messages[2].Content, "Processing")
	assert.NotNil(t, cmd)
	assert.True(t, finalM.thinking)
}

func TestInteractiveModel_InitAgentCmd_Extra(t *testing.T) {
	m := NewInteractiveModel(nil, "mock", "mock-model")
	cmd := m.initAgentCmd()
	msg := cmd()
	assert.NotNil(t, msg)
}

func TestInteractiveModel_SlashCommands_Extra(t *testing.T) {
	// Test slash command entry
	m := NewInteractiveModel(nil, "mock", "mock-model")

	// Type /
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}
	newM, _ := m.Update(msg)
	finalM := newM.(InteractiveModel)

	assert.True(t, finalM.showList)
	assert.Equal(t, ModeCmd, finalM.mode)
}

func TestInteractiveModel_BangShell_Extra(t *testing.T) {
	// Test shell command entry
	m := NewInteractiveModel(nil, "mock", "mock-model")

	// Type !
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}}
	newM, _ := m.Update(msg)
	finalM := newM.(InteractiveModel)

	assert.Equal(t, ModeShell, finalM.mode)
}
