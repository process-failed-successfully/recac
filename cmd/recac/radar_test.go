package main

import (
	"bytes"
	"context"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

// RadarTestMockAgent implements agent.Agent for testing
type RadarTestMockAgent struct {
	ResponseFunc func(prompt string) (string, error)
}

func (m *RadarTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.ResponseFunc != nil {
		return m.ResponseFunc(prompt)
	}
	return "[]", nil
}

func (m *RadarTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil {
		onChunk(resp)
	}
	return resp, err
}

func TestRadarRun(t *testing.T) {
	// Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "radar-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create go.mod
	goModContent := `module example.com/test
go 1.20

require (
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.8.0
)
`
	if err := os.WriteFile("go.mod", []byte(goModContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := &RadarTestMockAgent{
		ResponseFunc: func(prompt string) (string, error) {
			assert.Contains(t, prompt, "github.com/pkg/errors")
			assert.Contains(t, prompt, "github.com/stretchr/testify")

			// Return valid JSON
			return `[
				{
					"name": "github.com/pkg/errors",
					"quadrant": "Languages & Frameworks",
					"ring": "Adopt",
					"description": "Error handling primitives."
				},
				{
					"name": "github.com/stretchr/testify",
					"quadrant": "Tools",
					"ring": "Adopt",
					"description": "Assertion library."
				}
			]`, nil
		},
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Setup Command
	cmd := radarCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Set flags
	radarOutput = "test-radar.html"
	radarOpen = false // Don't try to open browser in test

	err = runRadar(cmd, []string{})
	assert.NoError(t, err)

	// Verify Output
	output := buf.String()
	assert.Contains(t, output, "Found 2 dependencies")
	assert.Contains(t, output, "Tech Radar generated: test-radar.html")

	// Verify HTML file creation
	htmlContent, err := os.ReadFile("test-radar.html")
	assert.NoError(t, err)
	assert.Contains(t, string(htmlContent), "github.com/pkg/errors")
	assert.Contains(t, string(htmlContent), "chart.js")
}

func TestRadarNoDeps(t *testing.T) {
	// Setup Temp Dir (Empty)
	tmpDir, err := os.MkdirTemp("", "radar-test-empty")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	cmd := radarCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = runRadar(cmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "No dependencies found")
}
