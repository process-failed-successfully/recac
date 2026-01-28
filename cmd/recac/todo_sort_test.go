package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestTodoSortCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-todo-sort-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Save current working directory and restore it after test
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	// Change to temp dir
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Helper to execute command
	execute := func(cmdArgs []string) (string, error) {
		rootCmd.SetArgs(cmdArgs)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	// Create a TODO.md with mixed tasks
	// Note: writeLines is available in the main package
	initialContent := []string{
		"# TODO",
		"",
		"- [ ] [main.go:10] NOTE: This is a note",
		"- [ ] [main.go:5] FIXME: This is a bug",
		"- [x] [main.go:1] TODO: Done task",
		"- [ ] [main.go:8] TODO: Standard task",
		"- [ ] [main.go:20] BUG: Another bug",
	}

	if err := writeLines("TODO.md", initialContent); err != nil {
		t.Fatal(err)
	}

	// Run Sort
	out, err := execute([]string{"todo", "sort"})
	if err != nil {
		t.Fatalf("Sort failed: %v\nOutput: %s", err, out)
	}

	// Read Result
	lines, err := readLines("TODO.md")
	if err != nil {
		t.Fatal(err)
	}

	// Expected Order:
	// 1. FIXME / BUG (Stable sort: FIXME comes before BUG in original list)
	// 2. TODO
	// 3. NOTE
	// 4. Done tasks

	// Filter out header and empty lines for checking
	var tasks []string
	for _, l := range lines {
		if strings.HasPrefix(l, "- [") {
			tasks = append(tasks, l)
		}
	}

	if len(tasks) != 5 {
		t.Fatalf("Expected 5 tasks, got %d", len(tasks))
	}

	// Debug print
	t.Logf("Tasks after sort:")
	for _, task := range tasks {
		t.Log(task)
	}

	// Check order
	// Index 0: FIXME (Priority 100). Was index 3 in original (line 5).
	// Index 1: BUG (Priority 100). Was index 6 in original (line 20).
	// Since slice is stable, FIXME should come before BUG if they have same priority?
	// Wait, original order:
	// 1. NOTE
	// 2. FIXME
	// 3. Done
	// 4. TODO
	// 5. BUG

	// Pending only:
	// 1. NOTE (10)
	// 2. FIXME (100)
	// 3. TODO (50)
	// 4. BUG (100)

	// Sorted Descending Priority:
	// FIXME (100) - came before BUG
	// BUG (100)
	// TODO (50)
	// NOTE (10)

	if !strings.Contains(tasks[0], "FIXME") {
		t.Errorf("Expected first task to be FIXME, got: %s", tasks[0])
	}
	if !strings.Contains(tasks[1], "BUG") {
		t.Errorf("Expected second task to be BUG, got: %s", tasks[1])
	}

	// Index 2 should be TODO (Priority 50)
	if !strings.Contains(tasks[2], "TODO") {
		t.Errorf("Expected third task to be TODO, got: %s", tasks[2])
	}

	// Index 3 should be NOTE (Priority 10)
	if !strings.Contains(tasks[3], "NOTE") {
		t.Errorf("Expected fourth task to be NOTE, got: %s", tasks[3])
	}

	// Index 4 should be Done task (regardless of priority, done goes to bottom)
	if !strings.Contains(tasks[4], "[x]") {
		t.Errorf("Expected last task to be Done ([x]), got: %s", tasks[4])
	}

	// Check output message
	if !strings.Contains(out, "Sorted 4 pending and 1 done tasks") {
		t.Errorf("Unexpected output message: %s", out)
	}
}
