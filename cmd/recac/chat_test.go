package main

import (
	"bytes"
	"context"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"
)

// MockChatAgent implements agent.Agent and records prompts
type MockChatAgent struct {
	LastPrompt string
	Response   string
}

func (m *MockChatAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.LastPrompt = prompt
	return m.Response, nil
}

func (m *MockChatAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	m.LastPrompt = prompt
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestHandleChatCommand_Persona(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	pm := agent.NewPersonaManager()
	// Ensure defaults are loaded (NewPersonaManager does this)
	p, _ := pm.GetPersona("default")

	session := &ChatSession{
		CurrentPersona: p,
		ContextFiles:   make(map[string]string),
		PM:             pm,
	}

	// 1. Switch to existing persona
	// "security" is a default persona
	res := handleChatCommand(cmd, session, "/persona security")
	if !res {
		t.Error("Expected command to be handled")
	}
	if session.CurrentPersona.Name != "Security Auditor" {
		t.Errorf("Expected persona to be Security Auditor, got %s", session.CurrentPersona.Name)
	}
	if !strings.Contains(out.String(), "Switched persona to: Security Auditor") {
		t.Errorf("Output mismatch: %s", out.String())
	}

	// 2. Switch to unknown persona
	out.Reset()
	res = handleChatCommand(cmd, session, "/persona unknown")
	if !res {
		t.Error("Expected command to be handled")
	}
	if session.CurrentPersona.Name != "Security Auditor" { // Should stay same
		t.Errorf("Expected persona to stay Security Auditor, got %s", session.CurrentPersona.Name)
	}
	if !strings.Contains(out.String(), "Unknown persona 'unknown'") {
		t.Errorf("Output mismatch: %s", out.String())
	}

	// 3. No args
	out.Reset()
	res = handleChatCommand(cmd, session, "/persona")
	if !res {
		t.Error("Expected command to be handled")
	}
	if !strings.Contains(out.String(), "Usage: /persona") {
		t.Errorf("Output mismatch: %s", out.String())
	}
}

func TestHandleChatCommand_Add(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	pm := agent.NewPersonaManager()
	p, _ := pm.GetPersona("default")

	session := &ChatSession{
		CurrentPersona: p,
		ContextFiles:   make(map[string]string),
		PM:             pm,
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "testfile.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("hello world")
	tmpFile.Close()

	// 1. Add file
	handleChatCommand(cmd, session, "/add "+tmpFile.Name())

	if content, ok := session.ContextFiles[tmpFile.Name()]; !ok {
		t.Error("File not added to context")
	} else if content != "hello world" {
		t.Errorf("Content mismatch. Got %s, want hello world", content)
	}

	// 2. Check output
	if !strings.Contains(out.String(), "Added") {
		t.Errorf("Expected 'Added' in output, got %s", out.String())
	}

	// 3. Add non-existent file
	out.Reset()
	errOut.Reset()
	handleChatCommand(cmd, session, "/add /nonexistent/file")
	if strings.Contains(out.String(), "Added") {
		t.Error("Should not add non-existent file")
	}
	if !strings.Contains(errOut.String(), "Failed to read file") {
		t.Errorf("Expected error message, got %s", errOut.String())
	}

	// 4. No args
	out.Reset()
	handleChatCommand(cmd, session, "/add")
	if !strings.Contains(out.String(), "Usage: /add") {
		t.Errorf("Output mismatch: %s", out.String())
	}
}

func TestHandleChatCommand_Clear(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// PM not needed for clear
	session := &ChatSession{
		History: "User: Hi\nAgent: Hello\n",
	}

	handleChatCommand(cmd, session, "/clear")
	if session.History != "" {
		t.Error("History not cleared")
	}
	if !strings.Contains(out.String(), "History cleared") {
		t.Errorf("Output mismatch: %s", out.String())
	}
}

func TestHandleChatCommand_Context(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	session := &ChatSession{ContextFiles: make(map[string]string)}

	// Empty
	handleChatCommand(cmd, session, "/context")
	if !strings.Contains(out.String(), "No files in context") {
		t.Error("Expected no files message")
	}

	// With files
	out.Reset()
	session.ContextFiles["foo.txt"] = "bar"
	handleChatCommand(cmd, session, "/context")
	if !strings.Contains(out.String(), "foo.txt") {
		t.Error("Expected file listing")
	}
}

func TestHandleChatCommand_Help(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	session := &ChatSession{}

	handleChatCommand(cmd, session, "/help")
	if !strings.Contains(out.String(), "Available commands") {
		t.Error("Expected help text")
	}
}

func TestHandleChatCommand_Quit(t *testing.T) {
	cmd := chatCmd
	session := &ChatSession{}
	if handleChatCommand(cmd, session, "/quit") {
		t.Error("Expected false for quit")
	}
	if handleChatCommand(cmd, session, "/exit") {
		t.Error("Expected false for exit")
	}
}

func TestHandleChatCommand_SaveLoad(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)

	pm := agent.NewPersonaManager()
	p, _ := pm.GetPersona("default")

	session := &ChatSession{
		CurrentPersona: p,
		ContextFiles:   map[string]string{"foo.txt": "bar"},
		History:        "User: Hello",
		PM:             pm,
	}

	tmpFile, err := os.CreateTemp("", "chat_session_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// 1. Test /save
	handleChatCommand(cmd, session, "/save "+tmpFile.Name())
	if !strings.Contains(out.String(), "Chat saved to") {
		t.Errorf("Expected save success message, got: %s", out.String())
	}

	// Verify file content
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "User: Hello") {
		t.Error("Saved JSON missing history")
	}

	// 2. Test /load
	out.Reset()
	newSession := &ChatSession{
		PM: pm,
	}
	handleChatCommand(cmd, newSession, "/load "+tmpFile.Name())
	if !strings.Contains(out.String(), "Chat loaded from") {
		t.Errorf("Expected load success message, got: %s", out.String())
	}

	// Verify loaded session
	if newSession.History != "User: Hello" {
		t.Errorf("Expected history 'User: Hello', got %s", newSession.History)
	}
	if newSession.ContextFiles["foo.txt"] != "bar" {
		t.Errorf("Expected context file 'foo.txt'='bar', got %v", newSession.ContextFiles)
	}
	if newSession.CurrentPersona.Name != "Default" {
		t.Errorf("Expected persona 'Default', got %s", newSession.CurrentPersona.Name)
	}

	// 3. Test load non-existent
	errOut.Reset()
	handleChatCommand(cmd, newSession, "/load /nonexistent/file")
	if !strings.Contains(errOut.String(), "Failed to read file") {
		t.Errorf("Expected error message, got %s", errOut.String())
	}
}

func TestHandleChatCommand_Exec(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	session := &ChatSession{
		History: "Initial",
	}

	// Execute a simple echo command
	handleChatCommand(cmd, session, "/exec echo mock output")
	if !strings.Contains(out.String(), "Executing: echo mock output") {
		t.Errorf("Expected executing message, got %s", out.String())
	}
	if !strings.Contains(out.String(), "Output added to context") {
		t.Errorf("Expected added message, got %s", out.String())
	}
	if !strings.Contains(session.History, "mock output") {
		t.Errorf("Expected 'mock output' in history, got %s", session.History)
	}
	if !strings.Contains(session.History, "echo mock output") {
		t.Errorf("Expected command 'echo mock output' in history, got %s", session.History)
	}
}

func TestHandleChatCommand_Unknown(t *testing.T) {
	cmd := chatCmd
	var out bytes.Buffer
	cmd.SetOut(&out)
	session := &ChatSession{}

	handleChatCommand(cmd, session, "/unknown")
	if !strings.Contains(out.String(), "Unknown command") {
		t.Error("Expected unknown command message")
	}
}

func TestRunChat_Integration(t *testing.T) {
	// Override factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := &MockChatAgent{Response: "Hello from Mock"}

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	cmd := chatCmd
	var out bytes.Buffer
	var in bytes.Buffer

	cmd.SetOut(&out)
	cmd.SetIn(&in)

	// Simulate user input
	in.WriteString("Hello\n")
	in.WriteString("/persona product\n")
	in.WriteString("How about now?\n")
	in.WriteString("/quit\n")

	// Run
	err := runChat(cmd, []string{})
	if err != nil {
		t.Fatalf("runChat failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "RECAC Chat Session Started") {
		t.Error("Missing welcome message")
	}
	if !strings.Contains(output, "Hello from Mock") {
		t.Error("Missing agent response")
	}
	if !strings.Contains(output, "Switched persona to: Product Manager") {
		t.Error("Missing persona switch message")
	}

	// Verify prompt
	// The last prompt should contain the last user input
	if !strings.Contains(mockAgent.LastPrompt, "How about now?") {
		t.Error("Last prompt missing user input")
	}
	// It should also contain history of previous turn
	if !strings.Contains(mockAgent.LastPrompt, "User: Hello") {
		t.Error("Last prompt missing history")
	}
	if !strings.Contains(mockAgent.LastPrompt, "Agent: Hello from Mock") {
		t.Error("Last prompt missing agent history")
	}
	// It should verify system prompt changed
	if !strings.Contains(mockAgent.LastPrompt, "Product Manager") {
		t.Error("Last prompt missing persona system prompt")
	}
}

func TestBuildChatPrompt(t *testing.T) {
	pm := agent.NewPersonaManager()
	p, _ := pm.GetPersona("default")
	session := &ChatSession{
		CurrentPersona: p,
		ContextFiles:   map[string]string{"foo.txt": "bar content"},
		History:        "User: A\nAgent: B\n",
		PM:             pm,
	}

	prompt := buildChatPrompt(session, "Current Input")

	if !strings.Contains(prompt, p.SystemPrompt) {
		t.Error("Missing system prompt")
	}
	if !strings.Contains(prompt, "--- foo.txt ---") {
		t.Error("Missing file header")
	}
	if !strings.Contains(prompt, "bar content") {
		t.Error("Missing file content")
	}
	if !strings.Contains(prompt, "Chat History:") {
		t.Error("Missing history header")
	}
	if !strings.Contains(prompt, "User: A") {
		t.Error("Missing history content")
	}
	if !strings.Contains(prompt, "User: Current Input") {
		t.Error("Missing current input")
	}
}
