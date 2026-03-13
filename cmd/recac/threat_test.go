package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// Define a concrete struct to implement the Agent interface for testing
type TestThreatAgent struct {
	Response string
}

func (m *TestThreatAgent) Send(ctx context.Context, prompt string) (string, error) {
	if !strings.Contains(prompt, "STRIDE") {
		return "", fmt.Errorf("prompt missing STRIDE keyword")
	}
	return m.Response, nil
}

func (m *TestThreatAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestThreatCmd(t *testing.T) {
	// Create temp directory for test files
	tempDir := t.TempDir()
	specFile := filepath.Join(tempDir, "app_spec.txt")
	err := os.WriteFile(specFile, []byte("A simple web app with a login page."), 0644)
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	// Mock AI response
	mockResponse := `{
  "system_description": "A simple web app",
  "threats": [
    {
      "id": "T-1",
      "category": "Spoofing",
      "component": "Login",
      "description": "Attacker impersonates user",
      "severity": "High",
      "mitigations": [
        { "description": "Use MFA", "status": "Proposed" }
      ]
    }
  ]
}`

	// Override the global factory variable
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	// Use our concrete type
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &TestThreatAgent{Response: mockResponse}, nil
	}

	t.Run("JSON Output", func(t *testing.T) {
		// Use a fresh command and buffer
		cmd := &cobra.Command{Use: "threat", RunE: runThreat}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// Set global flags used by runThreat
		threatJSON = true
		threatOutput = ""
		threatFile = specFile

		// Execute
		err := runThreat(cmd, []string{})
		assert.NoError(t, err)

		// Verify Output
		output := buf.String()
		assert.Contains(t, output, `"id": "T-1"`)
		assert.Contains(t, output, `"category": "Spoofing"`)
	})

	t.Run("Markdown File Output", func(t *testing.T) {
		cmd := &cobra.Command{Use: "threat", RunE: runThreat}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		outFile := filepath.Join(tempDir, "threat_report.md")

		// Set global flags
		threatJSON = false
		threatOutput = outFile
		threatFile = specFile

		// Execute
		err := runThreat(cmd, []string{})
		assert.NoError(t, err)

		// Verify File Created
		content, err := os.ReadFile(outFile)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "# Threat Model Report (STRIDE)")
		assert.Contains(t, string(content), "Attacker impersonates user")
	})
}

func TestPrintThreatTable(t *testing.T) {
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	report := ThreatReport{
		SystemDescription: "Test System",
		Threats: []Threat{
			{
				ID:          "T-1",
				Category:    "Spoofing",
				Component:   "Login",
				Description: "A very long description that should be truncated because it exceeds the max length",
				Severity:    "High",
			},
		},
	}

	printThreatTable(cmd, report)

	output := buf.String()
	assert.Contains(t, output, "Test System")
	assert.Contains(t, output, "Identified Threats: 1")
	assert.Contains(t, output, "ID   SEVERITY")
	assert.Contains(t, output, "A very long description that should be truncated because ...")
}
