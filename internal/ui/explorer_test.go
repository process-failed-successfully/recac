package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExplorerModel_Init(t *testing.T) {
	m, err := NewExplorerModel(".", nil, nil, nil)
	require.NoError(t, err)
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestExplorerModel_New(t *testing.T) {
	tempDir := t.TempDir()
	m, err := NewExplorerModel(tempDir, nil, nil, nil)
	require.NoError(t, err)

	// Check title contains path
	absPath, _ := filepath.Abs(tempDir)
	assert.Contains(t, m.list.Title, absPath)
}

func TestExplorerModel_Update_Resize(t *testing.T) {
	m, err := NewExplorerModel(".", nil, nil, nil)
	require.NoError(t, err)

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, _ := m.Update(msg)

	finalM := newM.(ExplorerModel)
	assert.Equal(t, 100, finalM.width)
	assert.Equal(t, 50, finalM.height)
}

func TestExplorerModel_View(t *testing.T) {
	tempDir := t.TempDir()
	// Create a dummy file
	os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("content"), 0644)

	m, err := NewExplorerModel(tempDir, nil, nil, nil)
	require.NoError(t, err)

	// Update with size to ensure viewport/list renders
	m, _ = NewExplorerModel(tempDir, nil, nil, nil)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(ExplorerModel)

	view := m.View()
	assert.Contains(t, view, "test.txt")
}

func TestExplorerModel_Update_EnterFile(t *testing.T) {
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("file content"), 0644)
	require.NoError(t, err)

	m, err := NewExplorerModel(tempDir, nil, nil, nil)
	require.NoError(t, err)

	// Set size
	tmpM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = tmpM.(ExplorerModel)

	// Ensure list is populated and select the file
	// Since list order depends on OS, we might need to filter or find index.
	// But with 1 file, it should be selected or selectable.

	// Simulate "enter" key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ := m.Update(msg)
	finalM := newM.(ExplorerModel)

	// Should be viewing file now
	assert.True(t, finalM.viewingFile)
	assert.Contains(t, finalM.viewport.View(), "file content")
	assert.Contains(t, finalM.statusMessage, "Viewing: test.txt")
}

func TestExplorerModel_Update_Analysis(t *testing.T) {
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	mockExplain := func(path string) (string, error) {
		return "Analysis Result", nil
	}

	m, err := NewExplorerModel(tempDir, mockExplain, nil, nil)
	require.NoError(t, err)

	// Set size
	tmpM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = tmpM.(ExplorerModel)

	// Trigger explain with 'e'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	newM, cmd := m.Update(msg)

	// cmd should trigger analysis
	assert.NotNil(t, cmd)

	// Execute cmd to get result msg
	resultMsg := cmd()
	assert.IsType(t, analysisMsg{}, resultMsg)

	// Update with result
	newM, _ = newM.Update(resultMsg)
	finalM := newM.(ExplorerModel)

	assert.True(t, finalM.viewingFile)
	assert.Contains(t, finalM.viewport.View(), "Analysis Result")
}

func TestExplorerModel_Update_AnalysisError(t *testing.T) {
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	mockExplain := func(path string) (string, error) {
		return "", errors.New("fail")
	}

	m, err := NewExplorerModel(tempDir, mockExplain, nil, nil)
	require.NoError(t, err)

	// Trigger explain with 'e'
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}
	newM, cmd := m.Update(msg)

	// Execute cmd
	resultMsg := cmd()

	// Update with result
	newM, _ = newM.Update(resultMsg)
	finalM := newM.(ExplorerModel)

	assert.Contains(t, finalM.statusMessage, "Error: fail")
	assert.False(t, finalM.viewingFile)
}

func TestExplorerModel_Update_ExitView(t *testing.T) {
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("content"), 0644)
	require.NoError(t, err)

	m, err := NewExplorerModel(tempDir, nil, nil, nil)
	require.NoError(t, err)

	// Enter viewing mode manually
	m.viewingFile = true

	// Hit Esc
	msg := tea.KeyMsg{Type: tea.KeyEsc}
	newM, _ := m.Update(msg)
	finalM := newM.(ExplorerModel)

	assert.False(t, finalM.viewingFile)
}
