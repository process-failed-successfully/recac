package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

type ResolveSpyAgent struct {
	Response string
}

func (s *ResolveSpyAgent) Send(ctx context.Context, prompt string) (string, error) {
	return s.Response, nil
}

func (s *ResolveSpyAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return s.Response, nil
}

func TestResolveCommand(t *testing.T) {
	// Setup Mock Agent
	mockAgent := &ResolveSpyAgent{Response: "Resolved Code"}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Test Case 1: Resolve specific file
	t.Run("Resolve specific file", func(t *testing.T) {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, "conflict.txt")
		content := `Before
<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch
After`
		err := os.WriteFile(filePath, []byte(content), 0644)
		assert.NoError(t, err)

		// We need to invoke runResolve directly or via Execute.
		// Since resolveCmd is global and has flags, we need to be careful.
		// Let's call runResolve directly to avoid flag parsing issues if possible,
		// but runResolve takes *cobra.Command.

		// Reset flags
		resolveCmd.Flags().Set("auto", "true")

		// Redirect stdout/stderr to suppress output
		oldStdout := resolveCmd.OutOrStdout()
		resolveCmd.SetOut(io.Discard)
		defer resolveCmd.SetOut(oldStdout)

		err = runResolve(resolveCmd, []string{filePath})
		assert.NoError(t, err)

		// Verify content
		resolvedContent, err := os.ReadFile(filePath)
		assert.NoError(t, err)

		expected := "Before\nResolved CodeAfter"
		assert.Equal(t, expected, string(resolvedContent))
	})

	// Test Case 2: Parse Conflict Block 3-way
	t.Run("Parse 3-way conflict", func(t *testing.T) {
		block := `<<<<<<< HEAD
Ours
||||||| merged common ancestors
Base
=======
Theirs
>>>>>>> branch`
		ours, theirs, err := parseConflictBlock(block)
		assert.NoError(t, err)
		assert.Equal(t, "Ours", ours)
		assert.Equal(t, "Theirs", theirs)
	})

	// Test Case 3: Parse Conflict Block 2-way
	t.Run("Parse 2-way conflict", func(t *testing.T) {
		block := `<<<<<<< HEAD
Ours
=======
Theirs
>>>>>>> branch`
		ours, theirs, err := parseConflictBlock(block)
		assert.NoError(t, err)
		assert.Equal(t, "Ours", ours)
		assert.Equal(t, "Theirs", theirs)
	})
}
