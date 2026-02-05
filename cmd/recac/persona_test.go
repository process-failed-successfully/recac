package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
)

func TestPersonaAddCmd(t *testing.T) {
	// Mock getWd
	tmpDir := t.TempDir()
	origGetWd := getWd
	getWd = func() (string, error) { return tmpDir, nil }
	defer func() { getWd = origGetWd }()

	// Mock askOneFunc
	origAskOne := askOneFunc
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		switch pt := p.(type) {
		case *survey.Input:
			if pt.Message == "Persona ID (e.g., 'guru'):" {
				*(response.(*string)) = "test-persona"
			} else if pt.Message == "Display Name (e.g., 'Go Guru'):" {
				*(response.(*string)) = "Test Persona"
			} else if pt.Message == "Description:" {
				*(response.(*string)) = "Description"
			}
		case *survey.Multiline:
			if pt.Message == "System Prompt:" {
				*(response.(*string)) = "You are a test."
			}
		default:
			return fmt.Errorf("unexpected prompt: %T", p)
		}
		return nil
	}
	defer func() { askOneFunc = origAskOne }()

	cmd := personaAddCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Run without args (interactive)
	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify file created
	path := filepath.Join(tmpDir, ".recac", "personas.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Personas file not created at %s", path)
	}

	// Verify content
	personas, _ := agent.LoadPersonas(path)
	if p, ok := personas["test-persona"]; !ok {
		t.Error("Persona not saved")
	} else {
		if p.Name != "Test Persona" {
			t.Errorf("Name mismatch: %s", p.Name)
		}
	}

	if !strings.Contains(out.String(), "added successfully") {
		t.Errorf("Output missing success message: %s", out.String())
	}
}

func TestPersonaRemoveCmd(t *testing.T) {
	tmpDir := t.TempDir()
	origGetWd := getWd
	getWd = func() (string, error) { return tmpDir, nil }
	defer func() { getWd = origGetWd }()

	// Create initial personas
	path := filepath.Join(tmpDir, ".recac", "personas.yaml")
	personas := map[string]agent.Persona{
		"test-persona": {Name: "Test"},
	}
	agent.SavePersonas(path, personas)

	cmd := personaRemoveCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	// Run
	if err := cmd.RunE(cmd, []string{"test-persona"}); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify
	loaded, _ := agent.LoadPersonas(path)
	if _, ok := loaded["test-persona"]; ok {
		t.Error("Persona not removed")
	}

	if !strings.Contains(out.String(), "removed") {
		t.Errorf("Output missing success message: %s", out.String())
	}
}

func TestPersonaListCmd(t *testing.T) {
	tmpDir := t.TempDir()
	origGetWd := getWd
	getWd = func() (string, error) { return tmpDir, nil }
	defer func() { getWd = origGetWd }()

	// Create initial personas
	path := filepath.Join(tmpDir, ".recac", "personas.yaml")
	personas := map[string]agent.Persona{
		"test-persona": {Name: "Test", Description: "Desc"},
	}
	agent.SavePersonas(path, personas)

	cmd := personaListCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{}); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	output := out.String()
	// Should contain default
	if !strings.Contains(output, "security") {
		t.Error("Missing default persona")
	}
	// Should contain custom
	if !strings.Contains(output, "test-persona") {
		t.Error("Missing custom persona")
	}
	if !strings.Contains(output, "(custom)") {
		t.Error("Missing (custom) label")
	}
}

func TestPersonaShowCmd(t *testing.T) {
	tmpDir := t.TempDir()
	origGetWd := getWd
	getWd = func() (string, error) { return tmpDir, nil }
	defer func() { getWd = origGetWd }()

	personas := map[string]agent.Persona{
		"test-persona": {Name: "Test", Description: "Desc", SystemPrompt: "Prompt"},
	}
	agent.SavePersonas(filepath.Join(tmpDir, ".recac", "personas.yaml"), personas)

	cmd := personaShowCmd
	var out bytes.Buffer
	cmd.SetOut(&out)

	if err := cmd.RunE(cmd, []string{"test-persona"}); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Name:         Test") {
		t.Error("Missing Name")
	}
	if !strings.Contains(output, "Prompt") {
		t.Error("Missing Prompt")
	}
}
