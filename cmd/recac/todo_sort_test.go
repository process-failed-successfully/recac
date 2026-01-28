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

	execute := func(cmdArgs []string) (string, error) {
		rootCmd.SetArgs(cmdArgs)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	t.Run("Sorts correctly", func(t *testing.T) {
		initialContent := `# TODO
This is a header.

- [ ] Task C (Normal)
- [x] Task A (Done)
- [ ] [main.go:1] FIXME: Critical bug
- [ ] [utils.go:2] TODO: Improve this
- [ ] [hack.go:3] HACK: Ugly fix
- [x] Task B (Done)
- [ ] Task D (Normal)
`
		err := os.WriteFile("TODO.md", []byte(initialContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		out, err := execute([]string{"todo", "sort"})
		if err != nil {
			t.Fatalf("Failed to sort: %v\nOutput: %s", err, out)
		}

		contentBytes, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		content := string(contentBytes)

		lines := strings.Split(strings.TrimSpace(content), "\n")
		var nonEmptyLines []string
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				nonEmptyLines = append(nonEmptyLines, strings.TrimSpace(l))
			}
		}

		expectedOrder := []string{
			"# TODO",
			"This is a header.",
			"- [ ] [main.go:1] FIXME: Critical bug",
			"- [ ] [utils.go:2] TODO: Improve this",
			"- [ ] [hack.go:3] HACK: Ugly fix",
			"- [ ] Task C (Normal)",
			"- [ ] Task D (Normal)",
			"- [x] Task A (Done)",
			"- [x] Task B (Done)",
		}

		if len(nonEmptyLines) != len(expectedOrder) {
			t.Errorf("Line count mismatch. Got %d, want %d.\nGot:\n%s", len(nonEmptyLines), len(expectedOrder), content)
		}

		for i, line := range nonEmptyLines {
			if i >= len(expectedOrder) {
				break
			}
			if line != expectedOrder[i] {
				t.Errorf("Line %d mismatch.\nGot:  %s\nWant: %s", i, line, expectedOrder[i])
			}
		}
	})

	t.Run("Preserves multiline tasks", func(t *testing.T) {
		initialContent := `# TODO
- [ ] Task 1
  Note for task 1
- [ ] Task 2
`
		err := os.WriteFile("TODO.md", []byte(initialContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = execute([]string{"todo", "sort"})
		if err != nil {
			t.Fatalf("Failed to sort: %v", err)
		}

		contentBytes, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		content := string(contentBytes)

		expected := `# TODO
- [ ] Task 1
  Note for task 1
- [ ] Task 2
`
		if strings.TrimSpace(content) != strings.TrimSpace(expected) {
			t.Errorf("Multiline task content changed.\nGot:\n%s\nWant:\n%s", content, expected)
		}
	})

	t.Run("Moves multiline task correctly", func(t *testing.T) {
		initialContent := `# TODO
- [x] Task Done
  Note for done task
- [ ] Task Not Done
`
		err := os.WriteFile("TODO.md", []byte(initialContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		_, err = execute([]string{"todo", "sort"})
		if err != nil {
			t.Fatalf("Failed to sort: %v", err)
		}

		contentBytes, err := os.ReadFile("TODO.md")
		if err != nil {
			t.Fatal(err)
		}
		content := string(contentBytes)

		expected := `# TODO
- [ ] Task Not Done
- [x] Task Done
  Note for done task
`
		if strings.TrimSpace(content) != strings.TrimSpace(expected) {
			t.Errorf("Multiline task move failed.\nGot:\n%s\nWant:\n%s", content, expected)
		}
	})
}
