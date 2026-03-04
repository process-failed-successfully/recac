package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

type mockAgentAddTags struct {
	Response string
}

func (m *mockAgentAddTags) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *mockAgentAddTags) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestAddTagsCmd_Stdout(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	testFile := filepath.Join(tmpDir, "model.go")
	testContent := "package models\n\ntype User struct {\n\tID int\n\tName string\n}\n"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	assert.NoError(t, err)

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentAddTags{
			Response: "```go\npackage models\n\ntype User struct {\n\tID int `json:\"id\"`\n\tName string `json:\"name\"`\n}\n```\n",
		}, nil
	}

	cmd := NewAddTagsCmd()
	cmd.SetArgs([]string{testFile, "--tags", "json"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "`json:\"id\"`")
	assert.Contains(t, output, "`json:\"name\"`")
}

func TestAddTagsCmd_InPlace(t *testing.T) {
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	testFile := filepath.Join(tmpDir, "model.go")
	testContent := "package models\n\ntype User struct {\n\tID int\n\tName string\n}\n"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	assert.NoError(t, err)

	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgentAddTags{
			Response: "```go\npackage models\n\ntype User struct {\n\tID int `json:\"id\" yaml:\"id\"`\n\tName string `json:\"name\" yaml:\"name\"`\n}\n```\n",
		}, nil
	}

	cmd := NewAddTagsCmd()
	cmd.SetArgs([]string{testFile, "--tags", "json,yaml", "--in-place"})

	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = cmd.Execute()
	assert.NoError(t, err)

	content, err := os.ReadFile(testFile)
	assert.NoError(t, err)

	output := string(content)
	assert.Contains(t, output, "`json:\"id\" yaml:\"id\"`")
	assert.Contains(t, output, "`json:\"name\" yaml:\"name\"`")
}
