package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

var (
	ctxCopy      bool
	ctxOutput    string
	ctxTokens    bool
	ctxTree      bool
	ctxMaxSize   int64
	ctxIgnore    []string
	ctxNoContent bool
)

var contextCmd = &cobra.Command{
	Use:   "context [paths...]",
	Short: "Generate a context dump of the codebase for LLMs",
	Long:  `Generates a comprehensive context dump of the specified paths (or current directory) suitable for pasting into an LLM. Includes a file tree and file contents.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		roots := args
		if len(roots) == 0 {
			roots = []string{"."}
		}

		var outputBuilder strings.Builder

		// Default ignores
		ignoreMap := map[string]bool{
			".git":         true,
			"node_modules": true,
			"vendor":       true,
			"dist":         true,
			"build":        true,
			".recac":       true,
			".idea":        true,
			".vscode":      true,
			"bin":          true,
			"obj":          true,
			"__pycache__":  true,
		}
		for _, ign := range ctxIgnore {
			ignoreMap[ign] = true
		}

		// 1. Generate Tree
		if ctxTree {
			outputBuilder.WriteString("# File Tree\n\n```\n")
			for _, root := range roots {
				tree, err := generateTree(root, ignoreMap)
				if err != nil {
					return fmt.Errorf("failed to generate tree for %s: %w", root, err)
				}
				outputBuilder.WriteString(tree)
			}
			outputBuilder.WriteString("```\n\n")
		}

		// 2. Collect and Print Files
		if !ctxNoContent {
			outputBuilder.WriteString("# File Contents\n\n")
			for _, root := range roots {
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
							// If not explicitly ignored, we still might want to skip commonly hidden things
							// But let's assume if it's not in ignoreMap, we traverse it?
							// Actually, let's skip all dot-dirs to be safe unless specified?
							// Standard practice: skip hidden dirs.
							return filepath.SkipDir
						}
						return nil
					}

					// Check if file is ignored
					if ignoreMap[d.Name()] {
						return nil
					}

					// Check if parent dir was hidden/ignored? WalkDir skips the dir so we are safe.

					// Skip binary/large files
					info, err := d.Info()
					if err != nil {
						return nil
					}
					if info.Size() > ctxMaxSize {
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

					// We want to show the path relative to the root argument,
					// but if the root argument was ".", we show "file.txt".
					// If the root argument was "src", we show "file.txt" (relative to src).
					// If the user provided multiple roots, this might be confusing.
					// But typically users provide paths they want to dump.
					// Let's stick to the relative path which is cleaner.
					// If the root is not "." and not absolute, it preserves the structure relative to CWD?
					// No, filepath.Rel(root, path) strips the root.

					// If I run `recac context src`, I get `main.go`.
					// If I run `recac context .`, I get `src/main.go`.

					// If I run `recac context /abs/path`, I get `file.txt`.

					// To preserve context, maybe we should use the path as walked, but relative to CWD?
					// filepath.WalkDir passes `path` which is joined with root.
					// So `path` is what we want, but relative to CWD.

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
					return fmt.Errorf("failed to walk %s: %w", root, err)
				}
			}
		}

		result := outputBuilder.String()

		// 3. Stats
		if ctxTokens {
			// Rough estimate: 4 chars = 1 token
			tokens := len(result) / 4
			fmt.Fprintf(cmd.ErrOrStderr(), "Estimated Tokens: %d\n", tokens)
		}

		// 4. Output
		if ctxCopy {
			if err := clipboard.WriteAll(result); err != nil {
				// Fallback or warning?
				// On headless systems this might fail.
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to copy to clipboard: %v\n", err)
			} else {
				fmt.Fprintln(cmd.ErrOrStderr(), "Context copied to clipboard!")
			}
		}

		if ctxOutput != "" {
			if err := os.WriteFile(ctxOutput, []byte(result), 0644); err != nil {
				return fmt.Errorf("failed to write output file: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Context written to %s\n", ctxOutput)
		}

		if !ctxCopy && ctxOutput == "" {
			fmt.Fprint(cmd.OutOrStdout(), result)
		}

		return nil
	},
}

func init() {
	contextCmd.Flags().BoolVarP(&ctxCopy, "copy", "c", false, "Copy output to clipboard")
	contextCmd.Flags().StringVarP(&ctxOutput, "output", "o", "", "Write output to file")
	contextCmd.Flags().BoolVarP(&ctxTokens, "tokens", "t", false, "Show estimated token count")
	contextCmd.Flags().BoolVarP(&ctxTree, "tree", "T", true, "Include file tree")
	contextCmd.Flags().BoolVar(&ctxNoContent, "no-content", false, "Exclude file contents (tree only)")
	contextCmd.Flags().Int64VarP(&ctxMaxSize, "max-size", "s", 1024*1024, "Max file size to include (bytes)")
	contextCmd.Flags().StringSliceVarP(&ctxIgnore, "ignore", "i", nil, "Additional ignore patterns (directories)")

	rootCmd.AddCommand(contextCmd)
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
			// Skip hidden files in tree too?
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

func isBinaryExt(ext string) bool {
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".jpg", ".png", ".gif", ".pdf", ".zip", ".tar", ".gz", ".iso":
		return true
	}
	return false
}

func isBinaryContent(content []byte) bool {
	limit := 512
	if len(content) < limit {
		limit = len(content)
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}
