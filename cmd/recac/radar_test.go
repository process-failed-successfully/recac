package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockRadarAgent implements agent.Agent
type MockRadarAgent struct {
	Response string
	Err      error
}

func (m *MockRadarAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, m.Err
}

func (m *MockRadarAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, m.Err
}

func TestRunRadar(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-radar-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// 2. Create Dummy Files
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test\ngo 1.21"), 0644)
	assert.NoError(t, err)

	err = os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"dependencies": {"react": "^18.0.0"}}`), 0644)
	assert.NoError(t, err)

	// 3. Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := `[
		{"name": "Go", "quadrant": "Languages & Frameworks", "ring": "Adopt", "description": "Core language"},
		{"name": "React", "quadrant": "Languages & Frameworks", "ring": "Trial", "description": "UI Library"}
	]`

	agentClientFactory = func(ctx context.Context, provider, model, cwd, project string) (agent.Agent, error) {
		return &MockRadarAgent{Response: mockResponse}, nil
	}

	// 4. Run Command
	// We need to chdir to tmpDir because radar uses os.Getwd()
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)

	cmd := &cobra.Command{}
	radarOut = "test-radar.html"
	radarJSON = false

	err = runRadar(cmd, []string{})
	assert.NoError(t, err)

	// 5. Verify Output
	content, err := os.ReadFile("test-radar.html")
	assert.NoError(t, err)
	html := string(content)

	assert.Contains(t, html, "Technology Radar")
	assert.Contains(t, html, "Go")
	assert.Contains(t, html, "React")
	assert.Contains(t, html, "Adopt")
	assert.Contains(t, html, "Trial")
}

func TestRunRadarJSON(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-radar-json-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/test"), 0644)
	assert.NoError(t, err)

	// 2. Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := `[{"name": "Go", "quadrant": "Languages", "ring": "Adopt"}]`
	agentClientFactory = func(ctx context.Context, provider, model, cwd, project string) (agent.Agent, error) {
		return &MockRadarAgent{Response: mockResponse}, nil
	}

	// 3. Run
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)

	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	radarJSON = true
	radarOut = "should-not-be-created.html"

	err = runRadar(cmd, []string{})
	assert.NoError(t, err)

	output := outBuf.String()
	assert.Contains(t, output, `"name": "Go"`)
	assert.Contains(t, output, `"ring": "Adopt"`)
}
