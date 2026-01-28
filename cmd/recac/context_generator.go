package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type ContextOptions struct {
	Roots      []string
	Ignore     []string
	MaxSize    int64
	NoContent  bool
	Tree       bool
	ShowTokens bool
}

// GenerateCodebaseContext generates a markdown string containing the file tree and contents
// of the specified roots, respecting ignore patterns and size limits.
func GenerateCodebaseContext(opts ContextOptions) (string, error) {
	if len(opts.Roots) == 0 {
		opts.Roots = []string{"."}
	}
	if opts.MaxSize == 0 {
		opts.MaxSize = 1024 * 1024 // Default 1MB
	}

	var outputBuilder strings.Builder

	// Default ignores
	defaultIgnores := DefaultIgnoreMap()
	ignoreMap := make(map[string]bool, len(defaultIgnores)+len(opts.Ignore))
	for k, v := range defaultIgnores {
		ignoreMap[k] = v
	}
	for _, ign := range opts.Ignore {
		ignoreMap[ign] = true
	}

	// 1. Generate Tree
	if opts.Tree {
		outputBuilder.WriteString("# File Tree\n\n```\n")
		for _, root := range opts.Roots {
			tree, err := generateTree(root, ignoreMap)
			if err != nil {
				return "", fmt.Errorf("failed to generate tree for %s: %w", root, err)
			}
			outputBuilder.WriteString(tree)
		}
		outputBuilder.WriteString("```\n\n")
	}

	// 2. Collect and Print Files
	if !opts.NoContent {
		outputBuilder.WriteString("# File Contents\n\n")
		for _, root := range opts.Roots {
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}

				// Skip root itself if it is "."
				if path == "." {
					return nil
				}

				if d.IsDir() {
					if ignoreMap[d.Name()] {
						return filepath.SkipDir
					}
					// Skip hidden dirs if they start with . and are not . (current dir)
					if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
						return filepath.SkipDir
					}
					return nil
				}

				// Check if file is ignored
				if ignoreMap[d.Name()] {
					return nil
				}

				// Skip hidden files
				if strings.HasPrefix(d.Name(), ".") {
					return nil
				}

				// Skip binary/large files
				info, err := d.Info()
				if err != nil {
					return nil
				}
				if info.Size() > opts.MaxSize {
					return nil
				}

				ext := strings.ToLower(filepath.Ext(path))
				if isBinaryExt(ext) {
					return nil
				}

				// Read file
				content, err := os.ReadFile(path)
				if err != nil {
					return nil // Skip unreadable
				}

				// Check for null bytes
				if isBinaryContent(content) {
					return nil
				}

				displayPath := path
				if abs, err := filepath.Abs(path); err == nil {
					if cwd, err := os.Getwd(); err == nil {
						if rel, err := filepath.Rel(cwd, abs); err == nil {
							displayPath = rel
						}
					}
				}

				outputBuilder.WriteString(fmt.Sprintf("## File: %s\n\n```%s\n%s\n```\n\n", displayPath, strings.TrimPrefix(ext, "."), string(content)))

				return nil
			})
			if err != nil {
				return "", fmt.Errorf("failed to walk %s: %w", root, err)
			}
		}
	}

	return outputBuilder.String(), nil
}

func generateTree(root string, ignoreMap map[string]bool) (string, error) {
	var sb strings.Builder
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if path == root {
			return nil
		}

		if ignoreMap[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasPrefix(d.Name(), ".") && d.Name() != "." && d.Name() != ".." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, _ := filepath.Rel(root, path)
		depth := strings.Count(relPath, string(os.PathSeparator))

		indent := strings.Repeat("  ", depth)

		sb.WriteString(fmt.Sprintf("%s%s\n", indent, d.Name()))
		return nil
	})

	return sb.String(), err
}
