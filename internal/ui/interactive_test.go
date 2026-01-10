package ui

import (
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
