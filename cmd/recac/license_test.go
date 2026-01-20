package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LicenseTestMockAgent implements agent.Agent
type LicenseTestMockAgent struct {
	SendFunc       func(ctx context.Context, prompt string) (string, error)
	SendStreamFunc func(ctx context.Context, prompt string, onChunk func(string)) (string, error)
}

func (m *LicenseTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, prompt)
	}
	return "mock response", nil
}

func (m *LicenseTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if m.SendStreamFunc != nil {
		return m.SendStreamFunc(ctx, prompt, onChunk)
	}
	// Default behavior
	chunk := "Unknown"
	onChunk(chunk)
	return chunk, nil
}

func TestParseGoMod(t *testing.T) {
	content := `module test
go 1.20
require (
	github.com/foo/bar v1.0.0
	example.com/pkg v0.1.0
)
require github.com/baz/qux v2.0.0
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	deps, err := parseGoMod(path)
	require.NoError(t, err)
	assert.Len(t, deps, 3)
	assert.Contains(t, deps, "github.com/foo/bar")
	assert.Contains(t, deps, "example.com/pkg")
	assert.Contains(t, deps, "github.com/baz/qux")
}

func TestLicenseCheckCmd(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module test
require (
	github.com/safe/lib v1.0.0
	github.com/risky/gpl v1.0.0
	github.com/unknown/pkg v1.0.0
)
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644))

	// Mock Agent
	mockAgent := &LicenseTestMockAgent{
		SendStreamFunc: func(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
			var resp string
			// Simulate Agent responses
			if strings.Contains(prompt, "github.com/safe/lib") {
				resp = "MIT"
			} else if strings.Contains(prompt, "github.com/risky/gpl") {
				resp = "GPL-3.0"
			} else {
				resp = "Unknown"
			}
			onChunk(resp)
			return resp, nil
		},
	}

	// Override factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// We use the global rootCmd for testing
	// executeCommand handles flag resetting

	t.Run("JSON Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "license", "check", tmpDir, "--json")
		require.NoError(t, err)

		assert.Contains(t, output, `"package": "github.com/safe/lib"`)
		assert.Contains(t, output, `"license": "MIT"`)
		assert.Contains(t, output, `"status": "allowed"`)

		assert.Contains(t, output, `"package": "github.com/risky/gpl"`)
		assert.Contains(t, output, `"license": "GPL-3.0"`)
		assert.Contains(t, output, `"status": "denied"`)
	})

	t.Run("Fail on Denied", func(t *testing.T) {
		// Clean cache to force re-check (or just reuse cache if consistent)
		// With cache, status is still denied, so it should fail.
		_, err := executeCommand(rootCmd, "license", "check", tmpDir, "--fail")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "denied licenses found")
	})

	t.Run("Verify Cache", func(t *testing.T) {
		cacheContent, err := os.ReadFile(filepath.Join(tmpDir, ".recac", "licenses.json"))
		require.NoError(t, err)
		assert.Contains(t, string(cacheContent), "github.com/safe/lib")
	})
}
