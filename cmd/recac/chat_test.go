package main

import (
	"bytes"
	"os"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
)

func TestHandleChatCommand_Persona(t *testing.T) {
	cmd := &cobra.Command{}
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
		t.Errorf("Expected Security Auditor, got %s", session.CurrentPersona.Name)
	}

	// 2. Switch to unknown
	res = handleChatCommand(cmd, session, "/persona unknown")
	if !res {
		t.Error("Expected command to be handled")
	}

	if session.CurrentPersona.Name != "Security Auditor" {
		t.Error("Expected persona NOT to change")
	}

	// 3. No args
	res = handleChatCommand(cmd, session, "/persona")
	if !res {
		t.Error("Expected command to be handled")
	}
}

func TestHandleChatCommand_Add(t *testing.T) {
	cmd := &cobra.Command{}
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
	tmpFile := t.TempDir() + "/test.txt"
	os.WriteFile(tmpFile, []byte("hello world"), 0644)

	res := handleChatCommand(cmd, session, "/add "+tmpFile)
	if !res {
		t.Error("Expected command to be handled")
	}

	if _, ok := session.ContextFiles[tmpFile]; !ok {
		t.Error("Expected file to be added to context")
	}
}

func TestHandleChatCommand_Clear(t *testing.T) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	// PM not needed for clear
	session := &ChatSession{
		History: "User: Hi\nAgent: Hello\n",
	}

	res := handleChatCommand(cmd, session, "/clear")
	if !res {
		t.Error("Expected command to be handled")
	}

	if session.History != "" {
		t.Error("Expected history to be cleared")
	}
}
