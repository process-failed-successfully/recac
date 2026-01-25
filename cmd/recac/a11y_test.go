package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestA11yScanner_Scan(t *testing.T) {
	scanner := NewA11yScanner("")

	tests := []struct {
		name          string
		file          string
		content       string
		expectedTypes []string
	}{
		{
			name: "Valid Img",
			file: "test.html",
			content: `<div><img src="valid.jpg" alt="Description"></div>`,
			expectedTypes: []string{},
		},
		{
			name: "Missing Alt",
			file: "bad.html",
			content: `<div><img src="oops.jpg"></div>`,
			expectedTypes: []string{"Missing Alt Text"},
		},
		{
			name: "Bad Click Handler",
			file: "Click.jsx",
			content: `<div onClick={handler}>Click me</div>`,
			expectedTypes: []string{"Click on Non-Interactive"},
		},
		{
			name: "Valid Button Click",
			file: "Button.jsx",
			content: `<button onClick={handler}>Click me</button>`,
			expectedTypes: []string{},
		},
		{
			name: "Positive Tabindex",
			file: "Tab.html",
			content: `<div tabindex="1">Bad order</div>`,
			expectedTypes: []string{"Positive Tabindex"},
		},
		{
			name: "Missing Href",
			file: "Link.html",
			content: `<a>Link text</a>`,
			expectedTypes: []string{"Missing Href"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := scanner.Scan(tt.file, tt.content)
			var foundTypes []string
			for _, f := range findings {
				foundTypes = append(foundTypes, f.Type)
			}

			if len(tt.expectedTypes) == 0 {
				assert.Empty(t, findings)
			} else {
				for _, exp := range tt.expectedTypes {
					assert.Contains(t, foundTypes, exp)
				}
			}
		})
	}
}

func TestA11yCmd_Run(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()

	// Create some files
	badFile := filepath.Join(tmpDir, "bad.html")
	err := os.WriteFile(badFile, []byte(`<img src="fail.png">`), 0644)
	require.NoError(t, err)

	goodFile := filepath.Join(tmpDir, "good.html")
	err = os.WriteFile(goodFile, []byte(`<img src="pass.png" alt="Pass">`), 0644)
	require.NoError(t, err)

	// Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse(`[
		{"type": "AI Suggestion", "description": "Contrast issue", "line": 1, "severity": "warning"}
	]`)

	agentClientFactory = func(ctx context.Context, provider, model, path, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Test Static Scan
	t.Run("Static Scan", func(t *testing.T) {
		// executeCommand uses global rootCmd where a11yCmd is registered.
		output, err := executeCommand(rootCmd, "a11y", tmpDir, "--json")
		require.NoError(t, err)

		// executeCommand merges stdout/stderr, so we skip the log line
		jsonStart := strings.Index(output, "[")
		require.NotEqual(t, -1, jsonStart, "Could not find JSON start")
		jsonOutput := output[jsonStart:]

		var findings []A11yFinding
		// Use Decoder to stop after the first JSON value, ignoring trailing log text
		err = json.NewDecoder(strings.NewReader(jsonOutput)).Decode(&findings)
		require.NoError(t, err, "Output should be JSON: "+output)

		assert.NotEmpty(t, findings)
		found := false
		for _, f := range findings {
			if f.Type == "Missing Alt Text" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find missing alt text")
	})

	// Test AI Scan
	t.Run("AI Scan", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "a11y", tmpDir, "--ai", "--json")
		require.NoError(t, err)

		jsonStart := strings.Index(output, "[")
		require.NotEqual(t, -1, jsonStart, "Could not find JSON start")
		jsonOutput := output[jsonStart:]

		var findings []A11yFinding
		// Use Decoder to stop after the first JSON value, ignoring trailing log text
		err = json.NewDecoder(strings.NewReader(jsonOutput)).Decode(&findings)
		require.NoError(t, err)

		foundAI := false
		for _, f := range findings {
			if strings.Contains(f.Type, "AI") {
				foundAI = true
				break
			}
		}
		assert.True(t, foundAI, "Should find AI suggestion")
	})
}

// Helper to avoid polluting global state
func NewRootCmdForTest() *cobra.Command {
	// In this codebase, commands are added to global rootCmd in init().
	// So we can reuse rootCmd but we need to reset flags.
	return rootCmd
}
