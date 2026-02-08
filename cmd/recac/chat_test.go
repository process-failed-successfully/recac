package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"
)

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
	// Should stay same (Security Auditor)
	if session.CurrentPersona.Name != "Security Auditor" {
		t.Errorf("Expected persona to stay Security Auditor, got %s", session.CurrentPersona.Name)
	}
	if !strings.Contains(out.String(), "Unknown persona 'unknown'") {
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
	// Note: The error is printed to ErrOrStderr, which we captured
	if !strings.Contains(errOut.String(), "Failed to read file") {
		t.Errorf("Expected error message, got '%s'", errOut.String())
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
}

func TestRunChat_Integration(t *testing.T) {
	t.Setenv("RECAC_PERSONAS_FILE", filepath.Join(t.TempDir(), "personas.yaml"))

	// Override factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Hello from Mock")

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
}
