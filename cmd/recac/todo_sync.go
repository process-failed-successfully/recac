package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var todoSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Synchronize TODO.md with the codebase",
	Long: `Scans the codebase for TODO comments and synchronizes them with TODO.md.
- Adds new TODOs found in the code.
- Updates line numbers for existing TODOs.
- Marks TODOs as done ([x]) in TODO.md if they are removed from the code.
- Preserves manual tasks added to TODO.md.`,
	RunE: runTodoSync,
}

func init() {
	todoCmd.AddCommand(todoSyncCmd)
}

// TodoEntry represents a line in TODO.md
type TodoEntry struct {
	OriginalLine string
	IsDone       bool
	IsAuto       bool      // True if it has [file:line] metadata
	File         string
	Line         int
	Keyword      string
	Content      string    // The text content
	Matched      bool      // Used during reconciliation
}

// Regex to parse a line in TODO.md
// Matches: - [ ] [file:line] Keyword: Content
// Capture groups:
// 1: x or space (Status)
// 2: file:line (Metadata) - optional
// 3: Keyword: Content (Rest)
var todoLineRegex = regexp.MustCompile(`^-\s*\[([ xX])\]\s*(?:\[([^]]+)\])?\s*(.*)$`)
var metadataRegex = regexp.MustCompile(`^([^:]+):(\d+)$`)

func runTodoSync(cmd *cobra.Command, args []string) error {
	// 1. Scan Codebase
	// Use "." as root
	scannedTodos, err := ScanForTodos(".")
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// 2. Read Existing TODO.md
	if err := ensureTodoFile(); err != nil {
		return err
	}
	lines, err := readLines(todoFile)
	if err != nil {
		return err
	}

	entries := parseTodoFile(lines)

	// 3. Reconcile
	entries, stats := reconcileTodos(entries, scannedTodos)

	// 4. Write back
	if err := writeTodoFile(entries); err != nil {
		return err
	}

	// 5. Report
	fmt.Fprintf(cmd.OutOrStdout(), "Sync complete.\n")
	fmt.Fprintf(cmd.OutOrStdout(), "- Added: %d\n", stats.Added)
	fmt.Fprintf(cmd.OutOrStdout(), "- Updated: %d\n", stats.Updated)
	fmt.Fprintf(cmd.OutOrStdout(), "- Completed (Removed from code): %d\n", stats.Completed)
	fmt.Fprintf(cmd.OutOrStdout(), "- Manual tasks preserved: %d\n", stats.Manual)

	return nil
}

func parseTodoFile(lines []string) []*TodoEntry {
	var entries []*TodoEntry
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		match := todoLineRegex.FindStringSubmatch(trimmed)

		entry := &TodoEntry{
			OriginalLine: line,
		}

		if match != nil {
			// Status
			entry.IsDone = strings.ToLower(match[1]) == "x"

			// Metadata [file:line]
			metadata := match[2]
			content := match[3]

			if metadata != "" {
				metaMatch := metadataRegex.FindStringSubmatch(metadata)
				if metaMatch != nil {
					entry.IsAuto = true
					entry.File = metaMatch[1]
					fmt.Sscanf(metaMatch[2], "%d", &entry.Line)

					// Attempt to split Keyword: Content
					// todo_scan uses: fmt.Sprintf("[%s:%d] %s: %s", ...)
					// So content usually starts with "Keyword: "
					parts := strings.SplitN(content, ": ", 2)
					if len(parts) == 2 {
						entry.Keyword = parts[0]
						entry.Content = parts[1]
					} else {
						entry.Content = content
					}
				}
			} else {
				// Manual task
				entry.Content = content
			}
		}

		entries = append(entries, entry)
	}
	return entries
}

type SyncStats struct {
	Added     int
	Updated   int
	Completed int
	Manual    int
}

func reconcileTodos(entries []*TodoEntry, scanned []TodoItem) ([]*TodoEntry, SyncStats) {
	stats := SyncStats{}

	// Map scanned items by signature for easy lookup
	// Signature: File + Content (Keyword is usually part of content in scanner logic, but let's be safe)
	// Actually, TodoItem has Keyword and Content separated.
	// We matched based on File + Content. Line number is what we want to update.
	scannedMap := make(map[string]*TodoItem)
	for i := range scanned {
		key := makeKey(scanned[i].File, scanned[i].Content)
		scannedMap[key] = &scanned[i]
	}

	// 1. Process existing entries
	for _, entry := range entries {
		if !entry.IsAuto {
			// Manual task - keep as is
			if strings.HasPrefix(strings.TrimSpace(entry.OriginalLine), "- [") {
				stats.Manual++
			}
			continue
		}

		// Try to find in scanned
		key := makeKey(entry.File, entry.Content)
		if found, ok := scannedMap[key]; ok {
			// Found!
			updated := false
			// Check if line changed
			if entry.Line != found.Line {
				entry.Line = found.Line
				updated = true
			}
			// Mark as Matched so we don't add it again
			entry.Matched = true

			// Also mark the scanned item as consumed (we can remove from map or use another tracker)
			delete(scannedMap, key)

			// Ensure it's not marked done if it was done before but reappeared?
			// If it's in code, it should be open [ ].
			if entry.IsDone {
				entry.IsDone = false // Re-open if found in code
				updated = true
			}

			if updated {
				stats.Updated++
			}
		} else {
			// Not found in code
			if !entry.IsDone {
				entry.IsDone = true
				stats.Completed++
			}
		}
	}

	// 2. Add remaining scanned items as new entries
	// The ones remaining in scannedMap are new
	// We need to maintain order of original file, so we usually append new ones at the end.
	// But `entries` is a slice of pointers, we can append to it.

	// Convert map back to slice to sort? Or just iterate. Map iteration is random.
	// Better to iterate original scanned list and check if in map.
	for _, item := range scanned {
		key := makeKey(item.File, item.Content)
		if _, ok := scannedMap[key]; ok {
			// It is still in map, meaning it wasn't matched
			newEntry := &TodoEntry{
				IsDone:  false,
				IsAuto:  true,
				File:    item.File,
				Line:    item.Line,
				Keyword: item.Keyword,
				Content: item.Content,
			}
			entries = append(entries, newEntry)
			stats.Added++
		}
	}

	return entries, stats
}

func makeKey(file, content string) string {
	return fmt.Sprintf("%s|%s", file, content)
}

func writeTodoFile(entries []*TodoEntry) error {
	f, err := os.Create(todoFile)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, entry := range entries {
		if entry.IsAuto {
			// Reconstruct line
			// - [x] [file:line] Keyword: Content
			mark := " "
			if entry.IsDone {
				mark = "x"
			}

			// Use Raw format style if we can, but we parsed it out.
			// Format: [File:Line] Keyword: Content
			// But we need to handle if Keyword is empty?
			// TodoItem always has Keyword.

			line := fmt.Sprintf("- [%s] [%s:%d] %s: %s\n", mark, entry.File, entry.Line, entry.Keyword, entry.Content)
			if entry.Keyword == "" {
				// Fallback if parsing failed somehow
				line = fmt.Sprintf("- [%s] [%s:%d] %s\n", mark, entry.File, entry.Line, entry.Content)
			}

			if _, err := f.WriteString(line); err != nil {
				return err
			}
		} else {
			// Manual entry or non-task line (header, etc)
			// Just write OriginalLine if it exists, or reconstruct
			if entry.OriginalLine != "" {
				if !strings.HasSuffix(entry.OriginalLine, "\n") {
					entry.OriginalLine += "\n"
				}
				if _, err := f.WriteString(entry.OriginalLine); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
