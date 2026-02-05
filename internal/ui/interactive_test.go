package ui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent implements agent.Agent interface
type MockAgent struct {
	mock.Mock
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	// Simulate streaming if needed
	if chunks, ok := args.Get(0).([]string); ok {
		for _, c := range chunks {
			onChunk(c)
		}
		return "", args.Error(1)
	}
	// Fallback standard return
	return args.String(0), args.Error(1)
}

func TestLoadModelsFromFile(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	validFile := filepath.Join(tmpDir, "models.json")
	invalidFile := filepath.Join(tmpDir, "invalid.json")

	// Valid JSON content
	validContent := `{
		"models": [
			{
				"name": "test-model",
				"displayName": "Test Model",
				"description": "A test model"
			}
		]
	}`
	err := os.WriteFile(validFile, []byte(validContent), 0644)
	assert.NoError(t, err)

	// Invalid JSON content
	err = os.WriteFile(invalidFile, []byte("{ invalid json "), 0644)
	assert.NoError(t, err)

	// Test case 1: Valid file
	models, err := loadModelsFromFile(validFile)
	assert.NoError(t, err)
	assert.Len(t, models, 1)
	assert.Equal(t, "Test Model", models[0].Name)
	assert.Equal(t, "test-model", models[0].Value)

	// Test case 2: File not found
	_, err = loadModelsFromFile("nonexistent.json")
	assert.Error(t, err)

	// Test case 3: Invalid JSON
	_, err = loadModelsFromFile(invalidFile)
	assert.Error(t, err)
}

func TestInteractiveModel_Update_ModeSwitching(t *testing.T) {
	m := NewInteractiveModel(nil, "gemini", "gemini-pro")

	// Test switching to /model mode via command
	m.textarea.SetValue("/model")
	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedM.(InteractiveModel) // Cast back

	// Should be in ModeModelSelect
	assert.Equal(t, ModeModelSelect, m.mode)
	assert.True(t, m.showList)
	// We might have a command (initAgentCmd if it triggered re-init, but /model just switches mode)
	// Actually, looking at code:
	// if v != "" && strings.HasPrefix(v, "/") { ... if cmdName == "/model" { ... return m, nil } }
	// wait, `NewInteractiveModel` adds default commands including /model.
	// The default command action sets mode.
	// So cmd should be nil from the action, or whatever the action returns.
	_ = cmd

	// Test Back key (Esc) to return to Chat
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedM.(InteractiveModel)
	assert.Equal(t, ModeChat, m.mode)
	assert.False(t, m.showList)

	// Test switching to /agent mode via command
	m.textarea.SetValue("/agent")
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedM.(InteractiveModel)
	assert.Equal(t, ModeAgentSelect, m.mode)
}

func TestInteractiveModel_Update_Selection(t *testing.T) {
	m := NewInteractiveModel(nil, "gemini", "gemini-pro")

	// Simulate Model Selection Mode
	m.setMode(ModeModelSelect)

	// Select first item
	// We need to ensure list has items. NewInteractiveModel should populate them.
	assert.NotEmpty(t, m.list.Items())

	// Select current item (Enter)
	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedM.(InteractiveModel)

	// Should switch back to Chat
	assert.Equal(t, ModeChat, m.mode)
	// Should return a command to re-init agent
	assert.NotNil(t, cmd)
}

func TestInteractiveModel_Update_AgentExecution(t *testing.T) {
	m := NewInteractiveModel(nil, "gemini", "gemini-pro")

	// Inject Mock Agent
	mockAgent := new(MockAgent)
	// Textarea adds a newline when Enter is pressed before we consume the value
	mockAgent.On("SendStream", mock.Anything, "Hello\n", mock.Anything).Return([]string{"Hi", " there"}, nil)

	// Manually set active agent via AgentReadyMsg
	updatedM, _ := m.Update(AgentReadyMsg{Agent: mockAgent})
	m = updatedM.(InteractiveModel)
	assert.NotNil(t, m.activeAgent)

	// Send "Hello"
	m.textarea.SetValue("Hello")
	updatedM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedM.(InteractiveModel)

	// Should clear textarea and start thinking
	assert.Equal(t, "", m.textarea.Value())
	assert.True(t, m.thinking)
	assert.NotNil(t, cmd)

	// Execute the command (generateResponse)
	msg := cmd()
	// Should return AgentStreamStartMsg
	streamStartMsg, ok := msg.(AgentStreamStartMsg)
	assert.True(t, ok)

	// Update with StreamStart
	updatedM, cmd = m.Update(streamStartMsg)
	m = updatedM.(InteractiveModel)
	assert.True(t, m.isStreaming)

	// Check channels are set
	assert.NotNil(t, m.chunkChan)

	// Wait for chunk command (waitForChunkMsg)
	// Since we are mocking SendStream to run in a goroutine (via generateResponse),
	// but generateResponse in the code spawns a goroutine:
	/*
		go func() {
			_, err := m.activeAgent.SendStream(...)
			...
			close(chkCh)
		}()
	*/
	// We need to ensure SendStream runs.
	// In my mock above:
	/*
		func (m *MockAgent) SendStream(...) {
			if chunks, ok := args.Get(0).([]string); ok {
				for _, c := range chunks {
					onChunk(c)
				}
			}
		}
	*/
	// The generateResponse goroutine will call this synchronously.
	// So by the time we get AgentStreamStartMsg, the goroutine is running.
	// We need to drain the channel.

	// Simulate receiving chunks via waitForChunkMsg
	// The command returned by AgentStreamStartMsg update is `m.waitForChunkMsg()`

	// 1. Chunk 1
	chunkCmd := cmd
	chunkMsg := chunkCmd() // This blocks on channel read

	agentChunk, ok := chunkMsg.(AgentChunkMsg)
	if assert.True(t, ok) {
		assert.Equal(t, "Hi", agentChunk.Content)
	}

	updatedM, cmd = m.Update(agentChunk)
	m = updatedM.(InteractiveModel)
	assert.Contains(t, m.currentMsgBuffer, "Hi")

	// 2. Chunk 2
	chunkCmd = cmd
	chunkMsg = chunkCmd()
	agentChunk, ok = chunkMsg.(AgentChunkMsg)
	if assert.True(t, ok) {
		assert.Equal(t, " there", agentChunk.Content)
	}

	updatedM, cmd = m.Update(agentChunk)
	m = updatedM.(InteractiveModel)
	assert.Contains(t, m.currentMsgBuffer, "Hi there")

	// 3. End of stream
	chunkCmd = cmd
	chunkMsg = chunkCmd()
	agentResp, ok := chunkMsg.(AgentResponseMsg) // Done returns ResponseMsg
	assert.True(t, ok)
	assert.Equal(t, "", agentResp.Content)

	updatedM, _ = m.Update(agentResp)
	m = updatedM.(InteractiveModel)
	assert.False(t, m.thinking)
	assert.False(t, m.isStreaming)

	// Verify history
	lastMsg := m.messages[len(m.messages)-1]
	assert.Equal(t, "Hi there", lastMsg.Content)
}

func TestInteractiveModel_View(t *testing.T) {
	m := NewInteractiveModel(nil, "gemini", "gemini-pro")

	// Set Window Size to ensure viewport renders correctly
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatedM.(InteractiveModel)

	// Just verify it renders something without panic
	view := m.View()
	assert.NotEmpty(t, view)
	// assert.Contains(t, view, "RECAC") // ASCII art might not match simple string
	assert.Contains(t, view, "Provider:")
	assert.Contains(t, view, "Gemini")
}

func TestInteractiveModel_Filtering(t *testing.T) {
	m := NewInteractiveModel(nil, "gemini", "gemini-pro")

	// Initialize commands
	assert.NotEmpty(t, m.commands)

	// Type "/"
	// We simulate typing properly
	m.textarea.SetValue("")
	updatedM, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = updatedM.(InteractiveModel)

	assert.True(t, m.showList)

	// Type "mod" (which matches /model)
	// We can just set the value and send a dummy key to trigger the filter logic
	m.textarea.SetValue("/mod")

	// Send a non-rune key to trigger Update but not change text (e.g. Up arrow which goes to list, but filtering happens before or during?)
	// Filtering happens:
	// default:
	//    m.textarea, tiCmd = m.textarea.Update(msg)
	//    if m.showList { val := m.textarea.Value(); ... }

	// So any key works. If I send Up, textarea ignores it (or moves cursor), but value remains /mod.
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updatedM.(InteractiveModel)

	// List should be filtered.
	// We can check list items.
	items := m.list.Items()
	assert.NotEmpty(t, items)
	// Should contain "model" but not "agent" (maybe)
	// Actually "agent" won't match "mod".

	foundModel := false
	for _, i := range items {
		if c, ok := i.(CommandItem); ok {
			if c.Name == "/model" {
				foundModel = true
			}
			if c.Name == "/agent" {
				assert.Fail(t, "Should not contain /agent when filtering /mod")
			}
		}
	}
	assert.True(t, foundModel)
}

// Mock list item for testing
type mockItem string
func (m mockItem) FilterValue() string { return string(m) }
