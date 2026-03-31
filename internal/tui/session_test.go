package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionModel_Update(t *testing.T) {
	mockAg := agent.NewMockAgent()
	m := NewSessionModel(mockAg, "default")

	// Test initial state
	assert.Empty(t, m.messages)
	assert.False(t, m.isLoading)
	assert.Equal(t, "Default", m.persona.Name)

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
	m := NewSessionModel(nil, "default")

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

func TestSessionModel_HandleCommand_Add(t *testing.T) {
	m := NewSessionModel(nil, "default")
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	err := os.WriteFile(fpath, []byte("content"), 0644)
	require.NoError(t, err)

	// /add <path>
	m.input.SetValue("/add " + fpath)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)

	assert.Contains(t, sessM.history, "Added")
	assert.Contains(t, sessM.contextFiles, fpath)
	assert.Equal(t, "content", sessM.contextFiles[fpath])
}

func TestSessionModel_HandleCommand_Context(t *testing.T) {
	m := NewSessionModel(nil, "default")

	// Empty context
	m.input.SetValue("/context")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "No files in context")

	// Add file manually to state
	sessM.contextFiles["foo.txt"] = "bar"

	// Check context list
	sessM.input.SetValue("/context")
	newM, _ = sessM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM = newM.(SessionModel)

	assert.Contains(t, sessM.history, "Current context files")
	assert.Contains(t, sessM.history, "foo.txt")
}

func TestSessionModel_HandleCommand_Persona(t *testing.T) {
	m := NewSessionModel(nil, "default")

	// List personas
	m.input.SetValue("/persona")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "Available personas")

	// Switch persona
	m.input.SetValue("/persona security")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM = newM.(SessionModel)
	assert.Equal(t, "Security Auditor", sessM.persona.Name)
	assert.Contains(t, sessM.history, "Switched persona to **Security Auditor**")

	// Unknown persona
	m.input.SetValue("/persona unknown")
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM = newM.(SessionModel)
	assert.Contains(t, sessM.history, "Unknown persona 'unknown'")
}

func TestSessionModel_View(t *testing.T) {
	m := NewSessionModel(nil, "default")

	// Simulate window size to init viewport/renderer
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(SessionModel)

	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "Welcome to RECAC")

	// Add history
	m.history = "**User**: Hello\n\nAssistant: Hi\n\n"
	// Force re-render
	newM, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(SessionModel)

	view = m.View()
	assert.Contains(t, view, "Hello")
	assert.Contains(t, view, "Persona: Default")
}

func TestSessionModel_BuildPrompt(t *testing.T) {
	m := NewSessionModel(nil, "default")
	m.contextFiles["a.txt"] = "A content"
	m.contextFiles["b.txt"] = "B content"
	m.history = "History..."

	prompt := m.buildPrompt()

	assert.Contains(t, prompt, m.persona.SystemPrompt)
	assert.Contains(t, prompt, "Context Files:")
	assert.Contains(t, prompt, "--- a.txt ---")
	assert.Contains(t, prompt, "A content")
	assert.Contains(t, prompt, "--- b.txt ---")
	assert.Contains(t, prompt, "B content")
	assert.Contains(t, prompt, "History...")
	assert.Contains(t, prompt, "Assistant:")

	// Check sorting (a comes before b)
	idxA := strings.Index(prompt, "a.txt")
	idxB := strings.Index(prompt, "b.txt")
	assert.True(t, idxA < idxB, "Context files should be sorted")
}

func TestSessionModel_UnknownCommand(t *testing.T) {
	m := NewSessionModel(nil, "default")
	m.input.SetValue("/unknown")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)

	assert.Contains(t, sessM.history, "Unknown command: /unknown")
}

func TestSessionModel_Init(t *testing.T) {
	m := NewSessionModel(nil, "default")
	cmd := m.Init()
	assert.NotNil(t, cmd) // Should return blink cmd
}

func TestStartSession_Error(t *testing.T) {
	// StartSession uses tea.NewProgram().Run().
	// Can't easily test without mock.
	// But minimal test that checks it exists.
}

func TestWaitForChunk(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "test chunk"

	cmd := waitForChunk(ch)
	msg := cmd()

	chunk, ok := msg.(chunkMsg)
	require.True(t, ok)
	assert.Equal(t, "test chunk", chunk.chunk)
	assert.Equal(t, (<-chan string)(ch), chunk.ch)

	close(ch)
	cmd = waitForChunk(ch)
	msg = cmd()

	_, ok = msg.(doneMsg)
	require.True(t, ok)
}

func TestSessionModel_FocusTransition(t *testing.T) {
	mockAg := agent.NewMockAgent()
	m := NewSessionModel(mockAg, "default")

	// Initially focused
	assert.True(t, m.input.Focused(), "Input should be focused initially")

	// Send message -> Loading -> Blurred
	m.input.SetValue("Hello")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)

	assert.True(t, sessM.isLoading, "Should be loading")
	assert.False(t, sessM.input.Focused(), "Input should be blurred during loading")

	// Request completes -> Done -> Focused
	newM, _ = sessM.Update(doneMsg{})
	sessM = newM.(SessionModel)

	assert.False(t, sessM.isLoading, "Should not be loading")
	assert.True(t, sessM.input.Focused(), "Input should be focused after completion")
}

func TestSessionModel_HandleCommand_ContextEmpty(t *testing.T) {
	m := NewSessionModel(nil, "default")
	m.input.SetValue("/context")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "No files in context")
}

func TestSessionModel_HandleCommand_AddEmpty(t *testing.T) {
	m := NewSessionModel(nil, "default")
	m.input.SetValue("/add")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "Usage: /add <file_path>")
}

func TestSessionModel_HandleCommand_AddInvalid(t *testing.T) {
	m := NewSessionModel(nil, "default")
	m.input.SetValue("/add invalid_file.txt")
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "Failed to read file")
}

func TestSessionModel_ClearCtrlL(t *testing.T) {
	m := NewSessionModel(nil, "default")
	m.history = "test history"
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlL})
	sessM := newM.(SessionModel)
	assert.Empty(t, sessM.history)
}

func TestSessionModel_Error(t *testing.T) {
	m := NewSessionModel(nil, "default")
	newM, _ := m.Update(errMsg(os.ErrNotExist))
	sessM := newM.(SessionModel)
	assert.Contains(t, sessM.history, "**Error**:")
}

func TestSessionModel_StartSession(t *testing.T) {
	// similar to explorer
	m := NewSessionModel(nil, "default")
	assert.NotNil(t, m.input)
}

func TestStartSession_RunReal(t *testing.T) {
	mockAg := agent.NewMockAgent()

	var in bytes.Buffer
	// Add esc to quit
	in.Write([]byte{27}) // esc key
	var out bytes.Buffer

	err := StartSession(mockAg, "default", tea.WithInput(&in), tea.WithOutput(&out))
	assert.NoError(t, err)
}
