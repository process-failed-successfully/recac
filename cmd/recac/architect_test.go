package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockArchitectAgent implements agent.Agent
type MockArchitectAgent struct {
	mock.Mock
}

func (m *MockArchitectAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockArchitectAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestGenerateArchitecture(t *testing.T) {
	ctx := context.Background()

	t.Run("Valid Response", func(t *testing.T) {
		mockAgent := new(MockArchitectAgent)
		jsonResp := `{
			"architecture.yaml": "components: []",
			"contracts.yaml": "contracts: []"
		}`
		// Agent response might contain markdown code blocks
		fullResp := "Here is the plan:\n```json\n" + jsonResp + "\n```"

		mockAgent.On("Send", ctx, mock.Anything).Return(fullResp, nil)

		files, err := generateArchitecture(ctx, mockAgent, "spec content")
		assert.NoError(t, err)
		assert.Equal(t, "components: []", files["architecture.yaml"])
		assert.Equal(t, "contracts: []", files["contracts.yaml"])
	})

	t.Run("Valid Response without Markdown", func(t *testing.T) {
		mockAgent := new(MockArchitectAgent)
		jsonResp := `{
			"architecture.yaml": "components: []"
		}`
		mockAgent.On("Send", ctx, mock.Anything).Return(jsonResp, nil)

		files, err := generateArchitecture(ctx, mockAgent, "spec content")
		assert.NoError(t, err)
		assert.Equal(t, "components: []", files["architecture.yaml"])
	})

    t.Run("Agent Error", func(t *testing.T) {
		mockAgent := new(MockArchitectAgent)
		mockAgent.On("Send", ctx, mock.Anything).Return("", assert.AnError)

		_, err := generateArchitecture(ctx, mockAgent, "spec content")
		assert.Error(t, err)
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		mockAgent := new(MockArchitectAgent)
		mockAgent.On("Send", ctx, mock.Anything).Return("not json", nil)

		_, err := generateArchitecture(ctx, mockAgent, "spec content")
		assert.Error(t, err)
	})
}

func TestBasePathFS_Stat(t *testing.T) {
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.txt")
    err := os.WriteFile(testFile, []byte("content"), 0644)
    assert.NoError(t, err)

    fs := &BasePathFS{Base: tmpDir}
    info, err := fs.Stat("test.txt")
    assert.NoError(t, err)
    assert.Equal(t, "test.txt", info.Name())

    _, err = fs.Stat("nonexistent")
    assert.Error(t, err)
}
