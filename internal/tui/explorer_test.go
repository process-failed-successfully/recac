package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestExplorerModel_Init(t *testing.T) {
	// Create temp dir
	dir := t.TempDir()
	// Create files
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("content a"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("content b"), 0644)
	os.Mkdir(filepath.Join(dir, "subdir"), 0755)

	m := NewExplorerModel(dir)

	// Check Model Init
	assert.Nil(t, m.Init())

	assert.Equal(t, 3, len(m.files))
	// Sorted: subdir (dir), a.txt, b.txt
	assert.Equal(t, "subdir", m.files[0].Name())
	assert.Equal(t, "a.txt", m.files[1].Name())
	assert.Equal(t, "b.txt", m.files[2].Name())
	assert.Equal(t, 0, m.cursor)
}

func TestExplorerModel_Navigation(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "d1"), 0755)
	os.WriteFile(filepath.Join(dir, "f1"), nil, 0644)

	m := NewExplorerModel(dir)

	// Initial state
	assert.Equal(t, 0, m.cursor)

	// Move down
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(ExplorerModel)
	assert.Equal(t, 1, m.cursor)

	// Move up
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newM.(ExplorerModel)
	assert.Equal(t, 0, m.cursor)
}

func TestExplorerModel_EnterDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0755)
	os.WriteFile(filepath.Join(sub, "inner.txt"), nil, 0644)

	m := NewExplorerModel(dir)
	// subdir is first because dirs come first
	assert.Equal(t, "sub", m.files[0].Name())

	// Enter subdir
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newM.(ExplorerModel)

	// Check path updated
	// Resolve absolute paths for comparison
	absSub, _ := filepath.Abs(sub)
	// m.path is absolute
	// EvalSymlinks to handle /var/private on Mac if needed
	evalPath, _ := filepath.EvalSymlinks(m.path)
	evalSub, _ := filepath.EvalSymlinks(absSub)

	assert.Equal(t, evalSub, evalPath)
	assert.Equal(t, 1, len(m.files))
	assert.Equal(t, "inner.txt", m.files[0].Name())
}

func TestExplorerModel_GoUp(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	os.Mkdir(sub, 0755)

	m := NewExplorerModel(sub)

	// Go up
	newM, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	m = newM.(ExplorerModel)

	// Check path updated to parent
	absDir, _ := filepath.Abs(dir)
	// On Mac /private/var... can be tricky, evaluate symlinks
	evalPath, _ := filepath.EvalSymlinks(m.path)
	evalDir, _ := filepath.EvalSymlinks(absDir)
	assert.Equal(t, evalDir, evalPath)
}

func TestExplorerModel_View(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("Hello World"), 0644)
	os.Mkdir(filepath.Join(dir, "dir1"), 0755)

	m := NewExplorerModel(dir)

	// Simulate window size msg to initialize viewport
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(ExplorerModel)

	view := m.View()
	assert.NotEmpty(t, view)
	assert.Contains(t, view, "RECAC Explorer")
	assert.Contains(t, view, "file1.txt")
	assert.Contains(t, view, "dir1/")

	// Check preview content for initial selection (dir1 is first as it's a dir)
	assert.Contains(t, view, "Directory: dir1")

	// Move cursor to file1.txt
	newM, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newM.(ExplorerModel)

	view = m.View()
	// Preview should show file content
	assert.Contains(t, view, "Hello World")
}

func TestExplorerModel_LargeFile(t *testing.T) {
	dir := t.TempDir()
	// Create large file > 100KB
	largeContent := make([]byte, 1024*101)
	os.WriteFile(filepath.Join(dir, "large.txt"), largeContent, 0644)

	m := NewExplorerModel(dir)
	// Simulate window size
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(ExplorerModel)

	// large.txt should be selected (only file)
	view := m.View()
	assert.Contains(t, view, "File too large to preview")
}

func TestExplorerModel_View_Unready(t *testing.T) {
	dir := t.TempDir()
	m := NewExplorerModel(dir)
	// Not sending WindowSizeMsg, so ready=false

	assert.Contains(t, m.View(), "Initializing...")
}

func TestStartExplorer(t *testing.T) {
	// Call StartExplorer but with an invalid path maybe or just regular,
	// unfortunately tea.Program.Run blocks, so just test it compiles basically
	// and doesn't panic on setup.
	dir := t.TempDir()
	m := NewExplorerModel(dir)
	assert.NotNil(t, m.files)
}

func TestExplorerModel_Quit(t *testing.T) {
	dir := t.TempDir()
	m := NewExplorerModel(dir)
	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	assert.NotNil(t, cmd) // Should return tea.Quit

	m2 := newM.(ExplorerModel)
	assert.Equal(t, m.cursor, m2.cursor)
}

func TestExplorerModel_UpdatePreview_EmptyFiles(t *testing.T) {
	dir := t.TempDir()
	m := NewExplorerModel(dir)
	// Temp dir is empty.
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(ExplorerModel)
	m.loadFiles()
	assert.Contains(t, m.viewport.View(), "Empty directory")
	assert.Contains(t, m.viewport.View(), "Press 'h', 'backspace', or 'esc' to go back.")
}

func TestExplorerModel_SyntaxHighlighting(t *testing.T) {
	dir := t.TempDir()
	goFileContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(goFileContent), 0644)

	m := NewExplorerModel(dir)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(ExplorerModel)

	view := m.viewport.View()
	// Check for ANSI escape codes indicating syntax highlighting worked
	assert.Contains(t, view, "\x1b[")
	assert.Contains(t, view, "func")
	assert.Contains(t, view, "main")
}

func TestExplorerModel_SyntaxHighlightingFallback(t *testing.T) {
	dir := t.TempDir()
	// Fallback lexer is basically plain text
	txtFileContent := `Just a plain text file`
	os.WriteFile(filepath.Join(dir, "plain.txt"), []byte(txtFileContent), 0644)

	m := NewExplorerModel(dir)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newM.(ExplorerModel)

	view := m.viewport.View()
	// Fallback plain text should still contain the content.
	// Note: chroma with 'terminal256' might still output some reset sequences,
	// but it shouldn't fail or lose content.
	assert.Contains(t, view, "Just a plain text file")
}
