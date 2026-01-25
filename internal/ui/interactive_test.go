package ui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewInteractiveModel(t *testing.T) {
	cmds := []SlashCommand{
		{Name: "/test", Description: "Test Command", Action: nil},
	}
	m := NewInteractiveModel(cmds, "", "")

	if len(m.commands) < 3 { // /test + /model + /agent
		t.Errorf("Expected at least 3 commands, got %d", len(m.commands))
	}

	foundModel := false
	foundAgent := false
	for _, c := range m.commands {
		if c.Name == "/model" {
			foundModel = true
		}
		if c.Name == "/agent" {
			foundAgent = true
		}
	}

	if !foundModel {
		t.Error("Expected /model command to be auto-added")
	}
	if !foundAgent {
		t.Error("Expected /agent command to be auto-added")
	}
}

func TestInteractiveModel_InitializationUX(t *testing.T) {
	// 1. Initialize
	m := NewInteractiveModel(nil, "", "")

	// Post-fix check: Expecting status message and thinking=true

	if !m.thinking {
		t.Error("UX: Expected agent to start in 'thinking' state during initialization to show spinner")
	}

	if m.statusMessage != "Initializing Agent..." {
		t.Errorf("UX: Expected status message to be 'Initializing Agent...', got '%s'", m.statusMessage)
	}

	// 2. Simulate AgentReadyMsg
	msg := AgentReadyMsg{}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.thinking {
		t.Error("UX: Expected 'thinking' to be false after agent is ready")
	}

	if m.statusMessage != "" {
		t.Errorf("UX: Expected status message to be cleared after agent ready, got '%s'", m.statusMessage)
	}
}

func TestInteractiveModel_Update_ModeSwitching(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")

	// Test switching to Cmd mode via Slash
	// We need to type "/" into textarea, then Update
	m.textarea.SetValue("/")
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}} // Doesn't matter much if Value is set, but let's simulate key
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	// Note: Our logic sets ModeCmd when we type / if empty, or we press / key
	if m.mode != ModeCmd {
		t.Errorf("Expected ModeCmd after typing /, got %v", m.mode)
	}
	if m.showList != true {
		t.Error("Expected showList to be true in ModeCmd")
	}

	// Test switching to Shell mode via Bang
	m.setMode(ModeChat)
	m.textarea.SetValue("") // Clear first

	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}}
	updatedM, _ = m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.mode != ModeShell {
		t.Errorf("Expected ModeShell after typing !, got %v", m.mode)
	}
}

func TestInteractiveModel_Update_CommandExecution(t *testing.T) {
	executed := false
	cmds := []SlashCommand{
		{
			Name:        "/exec",
			Description: "Executes",
			Action: func(m *InteractiveModel, args []string) tea.Cmd {
				executed = true
				return nil
			},
		},
	}
	m := NewInteractiveModel(cmds, "", "")

	// Type "/exec"
	m.textarea.SetValue("/exec")

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if !executed {
		t.Error("Expected command action to be executed")
	}
	if m.mode != ModeChat {
		t.Error("Expected to return to ModeChat after execution")
	}
}

func TestInteractiveModel_Update_AgentSelection(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")

	// Enter Agent Mode directly for test
	m.setMode(ModeAgentSelect)

	// Select "OpenAI" (Value: "openai")
	// We mimic selection by setting list index
	var targetIndex int
	for i, item := range m.list.Items() {
		if item.(AgentItem).Value == "openai" {
			targetIndex = i
			break
		}
	}
	m.list.Select(targetIndex)

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.currentAgent != "openai" {
		t.Errorf("Expected currentAgent to be openai, got %s", m.currentAgent)
	}

	// Check if default model execution happened (by checking history or model)
	// OpenAI default is gpt-4o
	if m.currentModel != "gpt-4o" {
		t.Errorf("Expected currentModel to switch to gpt-4o, got %s", m.currentModel)
	}
}

func TestInteractiveModel_Update_Filtering(t *testing.T) {
	// Explicitly define commands
	cmds := []SlashCommand{
		{Name: "/custom", Description: "Custom", Action: nil},
	}
	m := NewInteractiveModel(cmds, "", "")
	m.showList = true
	m.setMode(ModeCmd)

	// Trigger Update to run filtering logic
	// We want final value to be "/cus"
	// Current: ""
	// Send: "/cu" via SetValue, then 's' via Update
	m.textarea.SetValue("/cu")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)

	// Check list items
	items := m.list.Items()
	foundCustom := false
	foundAgent := false

	for _, item := range items {
		if cmd, ok := item.(CommandItem); ok {
			if cmd.Name == "/custom" {
				foundCustom = true
			}
			if cmd.Name == "/agent" {
				foundAgent = true
			}
		}
	}

	if !foundCustom {
		t.Errorf("List should contain /custom. Found %d items", len(items))
	}
	if foundAgent {
		t.Error("List should NOT contain /agent when filtered by /cus")
	}
}

func TestInteractiveModel_View(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")

	// Send Window Size
	// Must capture return value as Update is value receiver
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updatedM.(InteractiveModel)

	// Test View
	output := m.View()
	if len(output) == 0 {
		t.Error("View returned empty string")
	}

	if !strings.Contains(output, "Recac") && !strings.Contains(output, "RECAC") {
		t.Error("View should contain 'Recac'")
	}

	// Test Menu View
	m.setMode(ModeAgentSelect)
	// Update again to set list height for menu mode
	updatedM, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updatedM.(InteractiveModel)

	output = m.View()
	if !strings.Contains(output, "OpenAI") {
		t.Error("Agent menu should show agent options like OpenAI")
	}
}

func TestInputModes(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")

	m.setMode(ModeChat)
	if m.textarea.Prompt != " ❯ " {
		t.Error("Wrong prompt for Chat")
	}

	m.setMode(ModeCmd)
	if m.textarea.Prompt != "/ " {
		t.Error("Wrong prompt for Cmd")
	}

	m.setMode(ModeShell)
	if m.textarea.Prompt != "! " {
		t.Error("Wrong prompt for Shell")
	}
}

func TestHelperMethods(t *testing.T) {
	c := CommandItem{Name: "C", Desc: "D"}
	if c.FilterValue() != "C" {
		t.Error("CommandItem FilterValue fail")
	}
	if c.Title() != "C" {
		t.Error("CommandItem Title fail")
	}
	if c.Description() != "D" {
		t.Error("CommandItem Description fail")
	}

	m := ModelItem{Name: "M", Value: "V", DescriptionDetails: "D"}
	if m.FilterValue() != "M" {
		t.Error("ModelItem FilterValue fail")
	}
	if m.Title() != "M" {
		t.Error("ModelItem Title fail")
	}
	if m.Description() != "D" {
		t.Error("ModelItem Description fail")
	}

	a := AgentItem{Name: "A", Value: "V", DescriptionDetails: "D"}
	if a.FilterValue() != "A" {
		t.Error("AgentItem FilterValue fail")
	}
	if a.Title() != "A" {
		t.Error("AgentItem Title fail")
	}
	if a.Description() != "D" {
		t.Error("AgentItem Description fail")
	}

	k := keys
	if len(k.FullHelp()) == 0 {
		t.Error("FullHelp fail")
	}
}

func TestInteractiveModel_ModelListing(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	m.currentAgent = "gemini"
	// Force populating list
	m.setMode(ModeModelSelect)

	// Check items
	items := m.list.Items()
	if len(items) == 0 {
		t.Error("Model list should not be empty for gemini")
	}

	// Check content
	found := false
	for _, item := range items {
		if mod, ok := item.(ModelItem); ok {
			if strings.Contains(mod.Name, "Gemini") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("Model list should contain Gemini models")
	}

	// Test fallback
	m.currentAgent = "unknown_agent"
	m.setMode(ModeModelSelect)
	// Should fallback to gemini or empty? Implementation says fallback to gemini
	items = m.list.Items()
	if len(items) == 0 {
		t.Error("Model list should fallback and not be empty")
	}
}

func TestInteractiveModel_ToggleList(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	m.showList = false

	m.toggleList()
	if !m.showList {
		t.Error("toggleList should enable list")
	}

	m.toggleList()
	if m.showList {
		t.Error("toggleList should disable list")
	}
}

func TestInteractiveModel_Init(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	cmd := m.Init()
	if cmd == nil {
		t.Error("Init should return batch cmd")
	}
}

func TestInteractiveModel_ShellExecution(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	cmd := m.runShellCommand("echo hello")
	if cmd == nil {
		t.Error("Should return cmd")
	}

	// Execute the command function manually
	msg := cmd()
	if str, ok := msg.(shellOutputMsg); ok {
		// Output likely contains "hello"
		s := string(str)
		if !strings.Contains(s, "hello") {
			t.Errorf("Shell command output expected 'hello', got '%s'", s)
		}
	} else {
		t.Error("Should return shellOutputMsg")
	}
}

func TestInteractiveModel_Conversation(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	// Simulate agent ready to ensure normal chat behavior (not system status)
	m.thinking = false
	m.statusMessage = ""

	m.conversation("User Msg", true)
	if len(m.messages) <= 1 {
		t.Error("Messages should store user msg")
	}
	last := m.messages[len(m.messages)-1]
	if last.Role != RoleUser {
		t.Error("User message should have RoleUser")
	}
	if last.Content != "User Msg" {
		t.Errorf("Expected 'User Msg', got '%s'", last.Content)
	}

	m.conversation("Bot Msg", false)
	last = m.messages[len(m.messages)-1]
	if last.Role != RoleBot {
		t.Error("Bot message should have RoleBot")
	}
	if last.Content != "Bot Msg" {
		t.Errorf("Expected 'Bot Msg', got '%s'", last.Content)
	}
}

func TestInteractiveModel_Update_StatusExecution(t *testing.T) {
	cmds := []SlashCommand{
		{
			Name:        "/status",
			Description: "Show RECAC status",
			Action: func(m *InteractiveModel, args []string) tea.Cmd {
				return func() tea.Msg {
					return StatusMsg(GetStatus())
				}
			},
		},
	}
	m := NewInteractiveModel(cmds, "", "")

	m.textarea.SetValue("/status")

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updatedM, cmd := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if cmd == nil {
		t.Fatal("Expected a command to be returned")
	}

	statusMsg := cmd()
	if _, ok := statusMsg.(StatusMsg); !ok {
		t.Errorf("Expected StatusMsg, got %T", statusMsg)
	}

	updatedM, _ = m.Update(statusMsg)
	m = updatedM.(InteractiveModel)

	if len(m.messages) < 2 {
		t.Fatal("History should contain status message")
	}
	last := m.messages[len(m.messages)-1]
	if !strings.Contains(last.Content, "RECAC Status") {
		t.Error("Expected status message in history")
	}
}

// Mock Agent for testing
type MockAgent struct {
	Response string
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// SendStream should be blocking for the duration of the stream
	onChunk(m.Response)
	return m.Response, nil
}

// Needed to satisfy interface if it has more methods?
// Let's assume Agent interface only has Send/SendStream for now based on usage.
// If not, I'll fix compilation error.
// InteractiveModel uses activeAgent which is agent.Agent interface.

func TestInteractiveModel_GenerateResponse(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	mockAgent := &MockAgent{Response: "Hello"}
	m.activeAgent = mockAgent // Inject mock

	cmd := m.generateResponse("Hi")
	assertNotNil(t, cmd)

	msg := cmd()
	streamMsg, ok := msg.(AgentStreamStartMsg)
	if !ok {
		t.Fatalf("Expected AgentStreamStartMsg, got %T", msg)
	}

	// Verify channels
	if streamMsg.ChunkChan == nil {
		t.Error("ChunkChan is nil")
	}

	// Wait for chunk
	chunk := <-streamMsg.ChunkChan
	if chunk != "Hello" {
		t.Errorf("Expected 'Hello', got '%s'", chunk)
	}
}

func TestInteractiveModel_WaitForChunkMsg(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")

	// Setup channels
	chunkChan := make(chan string, 1)
	errChan := make(chan error, 1)

	m.chunkChan = chunkChan
	m.errChan = errChan

	// Case 1: Chunk received
	chunkChan <- "chunk"
	cmd := m.waitForChunkMsg()
	msg := cmd()

	chunkMsg, ok := msg.(AgentChunkMsg)
	if !ok {
		t.Fatalf("Expected AgentChunkMsg, got %T", msg)
	}
	if chunkMsg.Content != "chunk" {
		t.Errorf("Expected 'chunk', got '%s'", chunkMsg.Content)
	}

	// Case 2: Error received
	errChan <- errors.New("stream error")
	cmd = m.waitForChunkMsg()
	msg = cmd()

	errMsg, ok := msg.(AgentErrorMsg)
	if !ok {
		t.Fatalf("Expected AgentErrorMsg, got %T", msg)
	}
	if errMsg.Err.Error() != "stream error" {
		t.Errorf("Expected 'stream error', got '%s'", errMsg.Err.Error())
	}

	// Case 3: Stream closed (Done)
	close(chunkChan)
	cmd = m.waitForChunkMsg()
	msg = cmd()

	respMsg, ok := msg.(AgentResponseMsg)
	if !ok {
		t.Fatalf("Expected AgentResponseMsg (Done), got %T", msg)
	}
	if respMsg.Content != "" {
		t.Errorf("Expected empty content for done, got '%s'", respMsg.Content)
	}
}

// Re-implement helper since we can't depend on other file's helper if running package test?
// Wait, interactive_test.go is in same package.
// But to be safe and avoid conflict if I named it differently, I'll use standard checks.
// I used standard t.Error above.
// Only assertNotNil used in GenerateResponse.

func assertNotNil(t *testing.T, obj interface{}) {
	if obj == nil {
		t.Helper()
		t.Error("Expected not nil")
	}
}

func TestInteractiveModel_Update_TabCompletion(t *testing.T) {
	cmds := []SlashCommand{
		{Name: "/custom", Description: "Custom", Action: nil},
	}
	m := NewInteractiveModel(cmds, "", "")

	// Case 1: Empty input -> Toggle List
	msg := tea.KeyMsg{Type: tea.KeyTab}
	updatedM, _ := m.Update(msg)
	m = updatedM.(InteractiveModel)
	if !m.showList {
		t.Error("Tab on empty input should toggle list")
	}

	// Case 2: Partial match -> Complete
	m.setMode(ModeCmd) // Ensure we are in command mode context for completion
	m.textarea.SetValue("cust")
	updatedM, _ = m.Update(msg)
	m = updatedM.(InteractiveModel)

	if m.textarea.Value() != "custom" {
		t.Errorf("Expected completion to 'custom', got '%s'", m.textarea.Value())
	}
}

func TestInteractiveModel_Update_EnterChat(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	mockAgent := &MockAgent{Response: "Response"}
	m.activeAgent = mockAgent

	m.textarea.SetValue("Hello")
	msg := tea.KeyMsg{Type: tea.KeyEnter}

	updatedM, cmd := m.Update(msg)
	m = updatedM.(InteractiveModel)

	if !m.thinking {
		t.Error("Expected thinking state after Enter")
	}
	if cmd == nil {
		t.Error("Expected command to generate response")
	}
	// Verify textarea cleared
	if m.textarea.Value() != "" {
		t.Error("Expected textarea to be cleared")
	}
}

func TestInteractiveModel_Update_ListSelection(t *testing.T) {
	executed := false
	cmds := []SlashCommand{
		{
			Name: "/exec",
			Action: func(m *InteractiveModel, args []string) tea.Cmd {
				executed = true
				return nil
			},
		},
	}
	m := NewInteractiveModel(cmds, "", "")
	m.showList = true
	m.setMode(ModeCmd)

	// Select item (assuming /exec is in list, we filter for it)
	m.textarea.SetValue("/exec")
	// Trigger filter update
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	// Select first item
	m.list.Select(0)

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	m.Update(msg)

	if !executed {
		t.Error("Expected list selection to execute command")
	}
}

func TestInteractiveModel_ClearHistory(t *testing.T) {
	m := NewInteractiveModel(nil, "", "")
	m.conversation("Test Message", true)

	if len(m.messages) < 2 { // Welcome msg + Test Message
		t.Errorf("Expected messages to be stored, got %d", len(m.messages))
	}

	m.ClearHistory()

	if len(m.messages) == 0 {
		t.Error("History should contain clearance confirmation")
	}
	last := m.messages[len(m.messages)-1]
	if !strings.Contains(last.Content, "cleared") {
		t.Error("Expected clearance message")
	}
}

func TestInteractiveModel_LoadModelsFromFile(t *testing.T) {
	// 1. Create temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "models.json")

	jsonContent := `{
		"models": [
			{"name": "TestModel", "displayName": "Display Test", "description": "Desc"}
		]
	}`

	err := os.WriteFile(filePath, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Load
	models, err := loadModelsFromFile(filePath)
	if err != nil {
		t.Fatalf("Failed to load models: %v", err)
	}

	if len(models) != 1 {
		t.Errorf("Expected 1 model, got %d", len(models))
	}

	if models[0].Name != "Display Test" {
		t.Errorf("Expected Name 'Display Test', got '%s'", models[0].Name)
	}
	if models[0].Value != "TestModel" {
		t.Errorf("Expected Value 'TestModel', got '%s'", models[0].Value)
	}

	// 3. Test non-existent file
	_, err = loadModelsFromFile("non-existent.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}
