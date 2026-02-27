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

// Use TypeGenMockAgent to avoid conflict with MockAgent in other test files
type TypeGenMockAgent struct {
	SendFunc func(ctx context.Context, prompt string) (string, error)
}

func (m *TypeGenMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "", nil
}

func (m *TypeGenMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func (m *TypeGenMockAgent) GetState() interface{} {
	return nil
}

func (m *TypeGenMockAgent) SetState(state interface{}) error {
	return nil
}

func (m *TypeGenMockAgent) GetHistory() []agent.Message {
	return nil
}

func (m *TypeGenMockAgent) GetStateFilePath() string {
	return ""
}

func (m *TypeGenMockAgent) ClearMemory() error {
	return nil
}

func (m *TypeGenMockAgent) ApplyInstructions(instructions string) {
}

func TestTypeGenCmd(t *testing.T) {
	// Backup and restore original factory
	originalAgentClientFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentClientFactory }()

	// Mock agent that returns a markdown-wrapped struct
	mockResp := "```go\ntype UserPayload struct {\n\tID int `json:\"id\"`\n\tName string `json:\"name\"`\n}\n```"

	agentClientFactory = func(ctx context.Context, provider, model, cwd, project string) (agent.Agent, error) {
		return &TypeGenMockAgent{
			SendFunc: func(ctx context.Context, prompt string) (string, error) {
				return mockResp, nil
			},
		}, nil
	}

	// Create a temporary file to act as the JSON payload
	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "payload.json")
	err := os.WriteFile(jsonFile, []byte(`{"id": 1, "name": "Alice"}`), 0644)
	assert.NoError(t, err)

	cmd := NewTypeGenCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Set args to test generating a Go struct
	cmd.SetArgs([]string{jsonFile, "--lang", "go", "--name", "UserPayload"})

	err = cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	// Assert the output matches the cleaned code block (without ```go wrapper)
	assert.Contains(t, output, "type UserPayload struct")
	assert.Contains(t, output, "ID int `json:\"id\"`")
	assert.NotContains(t, output, "```go")
	assert.NotContains(t, output, "```\n")
}

func TestTypeGenCmd_EmptyInput(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty.json")
	err := os.WriteFile(emptyFile, []byte(""), 0644)
	assert.NoError(t, err)

	cmd := NewTypeGenCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{emptyFile})

	err = cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "input is empty")
}

func TestTypeGenCmd_Stdin(t *testing.T) {
	originalAgentClientFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentClientFactory }()

	mockResp := "```typescript\nexport interface UserPayload {\n\tid: number;\n\tname: string;\n}\n```"
	agentClientFactory = func(ctx context.Context, provider, model, cwd, project string) (agent.Agent, error) {
		return &TypeGenMockAgent{
			SendFunc: func(ctx context.Context, prompt string) (string, error) {
				return mockResp, nil
			},
		}, nil
	}

	cmd := NewTypeGenCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Create pipe for stdin
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	cmd.SetIn(r)

	// Write JSON to pipe
	go func() {
		w.Write([]byte(`{"id": 1, "name": "Alice"}`))
		w.Close()
	}()

	cmd.SetArgs([]string{"--lang", "typescript"})
	err = cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "export interface UserPayload")
}

func TestTypeGenCmd_MissingFile(t *testing.T) {
	cmd := NewTypeGenCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	cmd.SetArgs([]string{"does_not_exist.json"})
	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}
