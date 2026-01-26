package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// TodoItem represents a scanned TODO comment in the codebase.
type TodoItem struct {
	File    string
	Line    int
	Keyword string
	Content string
	Raw     string
}

var todoScanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan codebase for TODOs and add them to TODO.md",
	Long:  `Scans the specified path (defaults to current directory) for comments starting with TODO, FIXME, BUG, HACK, or NOTE and adds them to the TODO.md file.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}
		return scanAndAddTodos(cmd, root)
	},
}

func init() {
	// todoCmd is defined in todo.go
	todoCmd.AddCommand(todoScanCmd)
}

func scanAndAddTodos(cmd *cobra.Command, root string) error {
	tasks, err := ScanForTodos(root)
	if err != nil {
		return err
	}

	if len(tasks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No new TODOs found.")
		return nil
	}

	count, err := addTasksToTodoFile(tasks)
	if err != nil {
		return err
	}

	if count > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Added %d new tasks to TODO.md\n", count)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "No new tasks added (all duplicates).")
	}

	return nil
}

// Regex to catch TODOs.
// Matches: (//|#|<!--|--|/*) [whitespace] (TODO|FIXME|...) [optional: (stuff)] [whitespace|:] (content)
var todoRegex = regexp.MustCompile(`(?i)(\/\/|#|<!--|--|\/\*)\s*(TODO|FIXME|BUG|HACK|NOTE)(?:\(.*\))?[:\s]+(.*)`)

func ScanForTodos(root string) ([]TodoItem, error) {
	var tasks []TodoItem

	// Default ignores
	ignoreMap := DefaultIgnoreMap()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == root {
			// Don't skip the root itself if it's the start.
			// Also avoid checking ignoreMap for the root directory, so explicit scans work.
			return nil
		}

		if d.IsDir() {
			if ignoreMap[d.Name()] {
				return filepath.SkipDir
			}
			// Skip hidden dirs
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
				return filepath.SkipDir
			}
			return nil
		}

		if ignoreMap[d.Name()] {
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}

		// Skip binary files (by extension)
		ext := strings.ToLower(filepath.Ext(path))
		if isBinaryExt(ext) {
			return nil
		}

		// Check content for binary
		f, err := os.Open(path)
		if err != nil {
			return nil // Skip unreadable
		}
		defer f.Close()

		// Read first 512 bytes to check for binary
		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		if n > 0 && isBinaryContent(buf[:n]) {
			return nil
		}

		// Reset file pointer
		f.Seek(0, 0)

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			matches := todoRegex.FindStringSubmatch(strings.TrimSpace(line))
			if len(matches) > 3 {
				// matches[2] is keyword (TODO)
				// matches[3] is content

				keyword := strings.ToUpper(matches[2])
				content := strings.TrimSpace(matches[3])

				// Remove trailing comment closers like */ or -->
				content = strings.TrimSuffix(content, "*/")
				content = strings.TrimSuffix(content, "-->")
				content = strings.TrimSpace(content)

				if content == "" {
					continue
				}

				displayPath := path
				// Try to make path relative to CWD
				if cwd, err := os.Getwd(); err == nil {
					if rel, err := filepath.Rel(cwd, path); err == nil {
						displayPath = rel
					}
				}

				// Format: [File:Line] Keyword: Content
				raw := fmt.Sprintf("[%s:%d] %s: %s", displayPath, lineNum, keyword, content)

				tasks = append(tasks, TodoItem{
					File:    displayPath,
					Line:    lineNum,
					Keyword: keyword,
					Content: content,
					Raw:     raw,
				})
			}
		}

		return nil
	})

	return tasks, err
}

func addTasksToTodoFile(newTasks []TodoItem) (int, error) {
	if err := ensureTodoFile(); err != nil {
		return 0, err
	}

	lines, err := readLines(todoFile)
	if err != nil {
		return 0, err
	}

	existingTasks := make(map[string]bool)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- [ ] ") {
			existingTasks[strings.TrimPrefix(trimmed, "- [ ] ")] = true
		} else if strings.HasPrefix(trimmed, "- [x] ") {
			existingTasks[strings.TrimPrefix(trimmed, "- [x] ")] = true
		}
	}

	addedCount := 0
	f, err := os.OpenFile(todoFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	for _, item := range newTasks {
		if !existingTasks[item.Raw] {
			if _, err := f.WriteString(fmt.Sprintf("- [ ] %s\n", item.Raw)); err != nil {
				return addedCount, err
			}
			addedCount++
		}
	}

	return addedCount, nil
}
