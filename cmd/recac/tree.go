package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	treeDepth  int
	treeSort   string
	treeOnlyGo bool
)

var treeCmd = &cobra.Command{
	Use:   "tree [path]",
	Short: "List files in a tree structure with metadata",
	Long:  `List files in a tree-like format, annotated with size, last modified time, complexity (for Go files), and TODO count.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		// Normalize root
		root, err := filepath.Abs(root)
		if err != nil {
			return err
		}

		// Collect Metadata
		meta, err := collectMetaData(root)
		if err != nil {
			return err
		}

		// Print Tree
		return walkAndPrint(cmd, root, meta)
	},
}

func init() {
	rootCmd.AddCommand(treeCmd)
	treeCmd.Flags().IntVarP(&treeDepth, "depth", "d", -1, "Max depth to traverse")
	treeCmd.Flags().StringVar(&treeSort, "sort", "name", "Sort by: name, size, time")
	treeCmd.Flags().BoolVar(&treeOnlyGo, "only-go", false, "Only show Go files")
}

type FileMeta struct {
	Size       int64
	ModTime    time.Time
	Complexity int
	Todos      int
}

func collectMetaData(root string) (map[string]*FileMeta, error) {
	meta := make(map[string]*FileMeta)

	// Pre-fill with Complexity
	// runComplexityAnalysis handles walking and parsing Go files.
	// We run it unconditionally for now as it's fast enough and filters internally.
	{
		complexities, err := runComplexityAnalysis(root)
		if err != nil {
			// Don't fail the whole tree if complexity fails, but log it.
			fmt.Fprintf(os.Stderr, "Warning: Complexity analysis failed: %v\n", err)
		} else {
			for _, c := range complexities {
				// c.File is absolute or relative depending on how runComplexityAnalysis was called.
				// runComplexityAnalysis uses filepath.Walk, which preserves the root prefix.
				absPath, _ := filepath.Abs(c.File)
				if _, ok := meta[absPath]; !ok {
					meta[absPath] = &FileMeta{}
				}
				meta[absPath].Complexity += c.Complexity
			}
		}
	}

	// Pre-fill with Todos
	todos, err := ScanForTodos(root)
	if err != nil {
		// Don't fail the whole tree, but log it.
		fmt.Fprintf(os.Stderr, "Warning: TODO scan failed: %v\n", err)
	} else {
		for _, t := range todos {
			// ScanForTodos returns paths relative to CWD if possible, or relative to root.
			// To be safe, let's resolve to absolute.
			// t.File is display path. ScanForTodos logic:
			// if cwd, err := os.Getwd(); err == nil { if rel, err := filepath.Rel(cwd, path); err == nil { displayPath = rel } }
			// So we need to reverse this or just assume we can find the file.
			// Actually ScanForTodos returns TodoItem which has File field.
			// But the original path is not stored in TodoItem directly as absolute.
			// Wait, ScanForTodos iterates and creates TodoItem.
			// Let's try to match by resolving t.File to absolute.
			absPath, _ := filepath.Abs(t.File)
			if _, ok := meta[absPath]; !ok {
				meta[absPath] = &FileMeta{}
			}
			meta[absPath].Todos++
		}
	}

	return meta, nil
}

func walkAndPrint(cmd *cobra.Command, root string, meta map[string]*FileMeta) error {
	// We need a custom walker to handle sorting and tree printing
	return printDir(cmd, root, "", 0, meta)
}

func printDir(cmd *cobra.Command, path string, prefix string, currentDepth int, meta map[string]*FileMeta) error {
	if treeDepth >= 0 && currentDepth > treeDepth {
		return nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	// Filter
	var visible []fs.DirEntry
	ignore := DefaultIgnoreMap()
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") && e.Name() != "." {
			continue
		}
		if ignore[e.Name()] {
			continue
		}
		if treeOnlyGo && !e.IsDir() && !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		visible = append(visible, e)
	}

	// Sort
	sort.Slice(visible, func(i, j int) bool {
		// Sorting directories first usually looks better
		if visible[i].IsDir() && !visible[j].IsDir() {
			return true
		}
		if !visible[i].IsDir() && visible[j].IsDir() {
			return false
		}

		if treeSort == "size" {
			infoI, _ := visible[i].Info()
			infoJ, _ := visible[j].Info()
			return infoI.Size() > infoJ.Size()
		} else if treeSort == "time" {
			infoI, _ := visible[i].Info()
			infoJ, _ := visible[j].Info()
			return infoI.ModTime().After(infoJ.ModTime())
		}
		return visible[i].Name() < visible[j].Name()
	})

	for i, entry := range visible {
		isLast := i == len(visible)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Build metadata string
		absPath := filepath.Join(path, entry.Name())
		m := meta[absPath]
		if m == nil {
			m = &FileMeta{}
		}
		// Populate basic info if not present (only really need size/time if not gathered elsewhere)
		m.Size = info.Size()
		m.ModTime = info.ModTime()

		metaStr := formatMeta(m, entry.IsDir())
		fmt.Fprintf(cmd.OutOrStdout(), "%s%s%s%s\n", prefix, connector, entry.Name(), metaStr)

		if entry.IsDir() {
			childPrefix := prefix + "│   "
			if isLast {
				childPrefix = prefix + "    "
			}
			printDir(cmd, filepath.Join(path, entry.Name()), childPrefix, currentDepth+1, meta)
		}
	}
	return nil
}

func formatMeta(m *FileMeta, isDir bool) string {
	var parts []string

	if !isDir {
		parts = append(parts, formatSize(m.Size))
	}

	// Time relative
	parts = append(parts, formatRelativeTime(m.ModTime))

	if m.Complexity > 0 {
		parts = append(parts, fmt.Sprintf("C:%d", m.Complexity))
	}
	if m.Todos > 0 {
		parts = append(parts, fmt.Sprintf("T:%d", m.Todos))
	}

	if len(parts) == 0 {
		return ""
	}
	return fmt.Sprintf(" [%s]", strings.Join(parts, " | "))
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%dB", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 30 {
		return fmt.Sprintf("%dd ago", days)
	}
	if days < 365 {
		return fmt.Sprintf("%dmo ago", days/30)
	}
	return fmt.Sprintf("%dy ago", days/365)
}
