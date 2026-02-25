package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type MockArchitectAgent struct {
	Response string
	Err      error
}

func (m *MockArchitectAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.Err != nil {
		return "", m.Err
	}
	return m.Response, nil
}

func (m *MockArchitectAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestGenerateArchitecture(t *testing.T) {
	t.Run("Valid JSON Response", func(t *testing.T) {
		jsonResp := `
Here is the architecture:
` + "```json" + `
{
	"architecture.yaml": "components: []",
	"docs/api.md": "# API"
}
` + "```"

		mockAgent := &MockArchitectAgent{Response: jsonResp}
		files, err := generateArchitecture(context.Background(), mockAgent, "App Spec")
		require.NoError(t, err)
		assert.Len(t, files, 2)
		assert.Equal(t, "components: []", files["architecture.yaml"])
		assert.Equal(t, "# API", files["docs/api.md"])
	})

	t.Run("Invalid JSON Response", func(t *testing.T) {
		mockAgent := &MockArchitectAgent{Response: "Not JSON"}
		_, err := generateArchitecture(context.Background(), mockAgent, "App Spec")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "json parse error")
	})

	t.Run("Agent Error", func(t *testing.T) {
		mockAgent := &MockArchitectAgent{Err: assert.AnError}
		_, err := generateArchitecture(context.Background(), mockAgent, "App Spec")
		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)
	})
}

func TestBasePathFS(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "basepathfs_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	fs := &BasePathFS{Base: tmpDir}

	// Stat existing file
	info, err := fs.Stat("test.txt")
	require.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name())

	// Stat non-existing file
	_, err = fs.Stat("missing.txt")
	assert.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}
