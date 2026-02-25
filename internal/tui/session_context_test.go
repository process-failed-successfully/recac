package tui

import (
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionModel_ContextCommands(t *testing.T) {
	// Setup temporary directory and file
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(tempFile, []byte("Hello World"), 0644)
	require.NoError(t, err)

	m := NewSessionModel(agent.NewMockAgent())

	// Test /add command
	cmdInput := "/add " + tempFile
	m.input.SetValue(cmdInput)
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM := newM.(SessionModel)

	// Verify file was added to context
	// NOTE: contextFiles is not yet exported or defined, this test expects it to be added
	assert.Contains(t, sessM.contextFiles, tempFile)
	assert.Equal(t, "Hello World", sessM.contextFiles[tempFile])
	assert.Contains(t, sessM.history, "Added "+tempFile)

	// Test /context command
	sessM.input.SetValue("/context")
	newM, _ = sessM.Update(tea.KeyMsg{Type: tea.KeyEnter})
	sessM = newM.(SessionModel)

	// Verify context listing in history
	assert.Contains(t, sessM.history, "Current context files:")
	assert.Contains(t, sessM.history, tempFile)
}
