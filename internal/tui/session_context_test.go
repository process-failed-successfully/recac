package tui

import (
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionModel_ContextFiles(t *testing.T) {
	mockAg := agent.NewMockAgent()
	m := NewSessionModel(mockAg, "default")

	// Create temp file
	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	err := os.WriteFile(file1, []byte("content1"), 0644)
	require.NoError(t, err)

	// Add context file directly
	m.contextFiles[file1] = "content1"

	// Build prompt and verify content is included
	prompt := m.buildPrompt()
	assert.Contains(t, prompt, "file1.txt")
	assert.Contains(t, prompt, "content1")

	// Add another file
	file2 := filepath.Join(dir, "file2.txt")
	err = os.WriteFile(file2, []byte("content2"), 0644)
	require.NoError(t, err)

	m.handleCommand("/add " + file2)
	prompt = m.buildPrompt()
	assert.Contains(t, prompt, "file2.txt")
	assert.Contains(t, prompt, "content2")
}
