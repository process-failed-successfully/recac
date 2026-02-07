package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTodoSync(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalWd)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// 1. Create a file with TODOs
	codeContent := `package main
// TODO: Task 1
func main() {
	// FIXME: Critical bug
}
`
	err = os.WriteFile("main.go", []byte(codeContent), 0644)
	require.NoError(t, err)

	// 2. Run Sync (Initial)
	// We call runTodoSync directly or via command.
	// Since runTodoSync takes *cobra.Command which is mainly for printing, we can pass a dummy or use executeCommand wrapper if we want to capture output.
	// executeCommand is defined in test_helpers_test.go and available here.

	output, err := executeCommand(rootCmd, "todo", "sync")
	require.NoError(t, err)
	assert.Contains(t, output, "Added: 2")

	// Verify TODO.md
	todoContent, err := os.ReadFile("TODO.md")
	require.NoError(t, err)
	// lines := strings.Split(string(todoContent), "\n") // Unused

	// Expecting:
	// # TODO
	// - [ ] [main.go:2] TODO: Task 1
	// - [ ] [main.go:4] FIXME: Critical bug

	assert.Contains(t, string(todoContent), "[main.go:2] TODO: Task 1")
	assert.Contains(t, string(todoContent), "[main.go:4] FIXME: Critical bug")

	// 3. Update Line (Move TODOs)
	newCodeContent := `package main

// Inserted line
// TODO: Task 1
func main() {
	// Inserted line
	// FIXME: Critical bug
}
`
	err = os.WriteFile("main.go", []byte(newCodeContent), 0644)
	require.NoError(t, err)

	output, err = executeCommand(rootCmd, "todo", "sync")
	require.NoError(t, err)
	assert.Contains(t, output, "Updated: 2")

	todoContent, err = os.ReadFile("TODO.md")
	require.NoError(t, err)
	assert.Contains(t, string(todoContent), "[main.go:4] TODO: Task 1") // Shifted by 2
	assert.Contains(t, string(todoContent), "[main.go:7] FIXME: Critical bug") // Shifted by 3

	// 4. Mark Done (Remove TODO)
	finalCodeContent := `package main

// Inserted line
// TODO: Task 1
func main() {
	// Fixed bug
}
`
	err = os.WriteFile("main.go", []byte(finalCodeContent), 0644)
	require.NoError(t, err)

	output, err = executeCommand(rootCmd, "todo", "sync")
	require.NoError(t, err)
	assert.Contains(t, output, "Completed (Removed from code): 1")

	todoContent, err = os.ReadFile("TODO.md")
	require.NoError(t, err)
	assert.Contains(t, string(todoContent), "- [ ] [main.go:4] TODO: Task 1")
	// FIXME should be marked done [x]
	// Regex match for done task
	assert.Contains(t, string(todoContent), "- [x] [main.go:7] FIXME: Critical bug")

	// 5. Manual Task Preservation
	// Append a manual task
	f, err := os.OpenFile("TODO.md", os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = f.WriteString("- [ ] Manual task here\n")
	require.NoError(t, err)
	f.Close()

	output, err = executeCommand(rootCmd, "todo", "sync")
	require.NoError(t, err)
	assert.Contains(t, output, "Manual tasks preserved: 1")

	todoContent, err = os.ReadFile("TODO.md")
	require.NoError(t, err)
	assert.Contains(t, string(todoContent), "Manual task here")

	// 6. Re-open (Add back)
	// If I add back "FIXME: Critical bug", it should become [ ] again.
	// Note: We need to match content exactly "FIXME: Critical bug" (scanner parses Keyword: Content)
	// My sync logic matches File + Content.
	// Content for FIXME was "Critical bug".

	reopenCodeContent := `package main
// TODO: Task 1
// FIXME: Critical bug
`
	err = os.WriteFile("main.go", []byte(reopenCodeContent), 0644)
	require.NoError(t, err)

	output, err = executeCommand(rootCmd, "todo", "sync")
	require.NoError(t, err)

	// It might count as "Updated" because we toggle IsDone.
	assert.Contains(t, output, "Updated")

	todoContent, err = os.ReadFile("TODO.md")
	require.NoError(t, err)
	// Should be [ ] again
	assert.Contains(t, string(todoContent), "- [ ] [main.go:3] FIXME: Critical bug")
}

func TestParseTodoFile(t *testing.T) {
	lines := []string{
		"# TODO",
		"",
		"- [ ] [main.go:10] TODO: Task 1",
		"- [x] [utils.go:20] FIXME: Fix me",
		"- [ ] Manual task",
		"- [ ] [file.go:5] NOTE: Just a note",
	}

	entries := parseTodoFile(lines)
	assert.Equal(t, 6, len(entries))

	// Entry 2: Auto
	assert.True(t, entries[2].IsAuto)
	assert.False(t, entries[2].IsDone)
	assert.Equal(t, "main.go", entries[2].File)
	assert.Equal(t, 10, entries[2].Line)
	assert.Equal(t, "TODO", entries[2].Keyword)
	assert.Equal(t, "Task 1", entries[2].Content)

	// Entry 3: Done
	assert.True(t, entries[3].IsAuto)
	assert.True(t, entries[3].IsDone)

	// Entry 4: Manual
	assert.False(t, entries[4].IsAuto)
	assert.Equal(t, "Manual task", entries[4].Content)
}

func TestReconcileTodos(t *testing.T) {
	entries := []*TodoEntry{
		{File: "a.go", Line: 10, Content: "Foo", IsAuto: true},
		{File: "b.go", Line: 20, Content: "Bar", IsAuto: true, IsDone: true},
	}

	scanned := []TodoItem{
		// a.go moved to 12
		{File: "a.go", Line: 12, Content: "Foo", Keyword: "TODO"},
		// b.go reappeared (was done)
		{File: "b.go", Line: 22, Content: "Bar", Keyword: "FIXME"},
		// New
		{File: "c.go", Line: 5, Content: "New", Keyword: "TODO"},
	}

	entries, stats := reconcileTodos(entries, scanned)

	assert.Equal(t, 1, stats.Added) // c.go
	assert.Equal(t, 2, stats.Updated) // a.go (line), b.go (done->open + line)

	// Verify entries modification
	// a.go
	assert.Equal(t, 12, entries[0].Line)
	assert.True(t, entries[0].Matched)

	// b.go
	assert.Equal(t, 22, entries[1].Line)
	assert.False(t, entries[1].IsDone) // Re-opened

	// c.go should be appended
	assert.Equal(t, 3, len(entries))
	assert.Equal(t, "c.go", entries[2].File)
}
