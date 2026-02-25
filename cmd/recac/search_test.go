package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAgent for Search
type mockSearchAgent struct {
	phase1Response string
	phase2Response string
	err            error
}

func (m *mockSearchAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if strings.Contains(prompt, "File List") {
		return m.phase1Response, nil
	}
	if strings.Contains(prompt, "File Contents") {
		return m.phase2Response, nil
	}
	return "", fmt.Errorf("unexpected prompt: %s", prompt)
}

func (m *mockSearchAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestSearchCmd_Success(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-search-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create dummy files
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\n\nfunc main() {\n  fmt.Println(\"Hello\")\n}\n"), 0644)
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(tmpDir, "pkg"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "pkg", "utils.go"), []byte("package utils\n\nfunc Helper() {}\n"), 0644)
	require.NoError(t, err)

	// Switch to tmp dir
	oldCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldCwd)

	// 2. Setup Mock Agent
	mockAg := &mockSearchAgent{
		phase1Response: `["main.go"]`,
		phase2Response: `[
			{
				"file": "main.go",
				"line": 4,
				"snippet": "fmt.Println(\"Hello\")",
				"reason": "Matches query"
			}
		]`,
	}

	// 3. Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 4. Run Command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = runSearch(cmd, []string{"hello"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Scanning file tree...")
	assert.Contains(t, output, "Identifying relevant files")
	assert.Contains(t, output, "Checking 1 files: [main.go]")
	assert.Contains(t, output, "Found Matches:")
	assert.Contains(t, output, "main.go:4")
	assert.Contains(t, output, "fmt.Println(\"Hello\")")
}

func TestSearchCmd_NoFilesFound(t *testing.T) {
	// 1. Setup Empty Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-search-test-empty")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	oldCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldCwd)

	// 2. Run Command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = runSearch(cmd, []string{"query"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no searchable files found")
}

func TestSearchCmd_AgentNoRelevantFiles(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-search-test-norelevant")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(filepath.Join(tmpDir, "file.md"), []byte("content"), 0644)
	require.NoError(t, err)

	oldCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldCwd)

	// 2. Setup Mock Agent
	mockAg := &mockSearchAgent{
		phase1Response: `[]`,
	}

	// 3. Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 4. Run Command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = runSearch(cmd, []string{"query"})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No relevant files identified")
}

func TestSearchCmd_AgentJSONError(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-search-test-jsonerror")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	require.NoError(t, err)

	oldCwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldCwd)

	// 2. Setup Mock Agent
	mockAg := &mockSearchAgent{
		phase1Response: `invalid-json`,
	}

	// 3. Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 4. Run Command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = runSearch(cmd, []string{"query"})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Agent identified files that do not exist")
}
