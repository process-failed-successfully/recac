package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewExplorerModel(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dir1"), 0755)

	m, err := NewExplorerModel(tmpDir, nil, nil, nil)
	assert.NoError(t, err)

	// Check items loaded
	// loadItems sorts dirs first.
	items := m.list.Items()
	assert.Equal(t, 2, len(items))

	// First item should be dir1
	assert.Equal(t, "dir1", items[0].(FileItem).Name)
	assert.True(t, items[0].(FileItem).IsDir)

	// Second item should be file1.txt
	assert.Equal(t, "file1.txt", items[1].(FileItem).Name)
	assert.False(t, items[1].(FileItem).IsDir)
}

func TestExplorerNavigation(t *testing.T) {
    tmpDir := t.TempDir()
    // Make sure we get absolute path for tmpDir to match internal logic
    tmpDir, _ = filepath.Abs(tmpDir)

    subdir := filepath.Join(tmpDir, "subdir")
    os.Mkdir(subdir, 0755)

    m, err := NewExplorerModel(tmpDir, nil, nil, nil)
    assert.NoError(t, err)

    // Select the directory (should be first)
    m.list.Select(0)

    // Send Enter
    newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

    // Cast back
    model := newM.(ExplorerModel)

    // Path should be updated
    assert.Equal(t, subdir, model.currentPath)

    // Test going back
    newM2, _ := model.Update(tea.KeyMsg{Type: tea.KeyBackspace})
    model2 := newM2.(ExplorerModel)
    assert.Equal(t, tmpDir, model2.currentPath)
}

func TestExplorerAnalysisTrigger(t *testing.T) {
    tmpDir := t.TempDir()
    file := filepath.Join(tmpDir, "test.go")
    os.WriteFile(file, []byte("package main"), 0644)

    called := false
    mockExplain := func(path string) (string, error) {
        called = true
        return "Explained", nil
    }

    m, err := NewExplorerModel(tmpDir, mockExplain, nil, nil)
    assert.NoError(t, err)

    // Set size to ensure viewport renders
    updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
    m = updatedM.(ExplorerModel)

    // Select the file
    m.list.Select(0)

    // Send 'e'
    newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
    model := newM.(ExplorerModel)

    // Check status
    assert.Contains(t, model.statusMessage, "Asking AI")

    // Check cmd execution
    if cmd != nil {
        msg := cmd()
        assert.IsType(t, analysisMsg{}, msg)
        assert.True(t, called)

        // Handle result msg
        finalM, _ := model.Update(msg)
        finalModel := finalM.(ExplorerModel)
        assert.True(t, finalModel.viewingFile)
        // Check content via model state, not View() output which has formatting
        // Actually viewport content is private? No, we can inspect it via reflection or if it has public method.
        // Viewport has no GetContent.
        // But we can check `View()` string contains "Explained"
        // Also need to set size on finalModel if it was lost (it shouldn't be)
        assert.Contains(t, finalModel.viewport.View(), "Explained")
    } else {
        t.Fatal("Expected command to be returned")
    }
}
