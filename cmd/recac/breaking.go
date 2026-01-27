package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/breaking"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	breakingBase string
	breakingPath string
	breakingJSON bool
	breakingFail bool
)

var breakingCmd = &cobra.Command{
	Use:   "breaking",
	Short: "Detect breaking changes in Go API",
	Long:  `Compares the public Go API of the current directory against a git reference (e.g., main, v1.0.0).
Reports removed or changed exported identifiers.`,
	RunE: runBreaking,
}

func init() {
	rootCmd.AddCommand(breakingCmd)
	breakingCmd.Flags().StringVar(&breakingBase, "base", "main", "Git reference to compare against")
	breakingCmd.Flags().StringVar(&breakingPath, "path", ".", "Path to analyze")
	breakingCmd.Flags().BoolVar(&breakingJSON, "json", false, "Output results as JSON")
	breakingCmd.Flags().BoolVar(&breakingFail, "fail", false, "Exit with error code if breaking changes found")
}

func runBreaking(cmd *cobra.Command, args []string) error {
	gitClient := gitClientFactory()

	// Get Repo Root
	repoRoot, err := gitClient.Run(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("failed to get git repo root: %w", err)
	}
	repoRoot = strings.TrimSpace(repoRoot)

	// 1. Get List of Go Files in HEAD (local)
	// We can walk the directory, ignoring common paths
	var localFiles []string

	// Ensure breakingPath is clean
	rootPath := filepath.Clean(breakingPath)

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			ignores := DefaultIgnoreMap()
			if ignores[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			// Normalize path to be relative to repoRoot
			absPath, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(repoRoot, absPath)
			if err != nil {
				// If file is outside repo (unlikely if walking subdir), just use path?
				relPath = path
			}
			localFiles = append(localFiles, relPath)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan local files: %w", err)
	}

	// 2. Extract Local API
	localLoader := func(p string) ([]byte, error) {
		// p is relative to repoRoot, we need absolute or CWD-relative path to read
		return os.ReadFile(filepath.Join(repoRoot, p))
	}
	localAPI, err := breaking.ExtractAPI(localFiles, localLoader)
	if err != nil {
		return fmt.Errorf("failed to extract local API: %w", err)
	}

	// 3. Get List of Go Files in Base (Git)
	// git ls-tree -r --name-only <base> <path>
	cmdArgs := []string{"ls-tree", "-r", "--name-only", breakingBase}
	if rootPath != "." {
		cmdArgs = append(cmdArgs, rootPath)
	}

	out, err := gitClient.Run(repoRoot, cmdArgs...)
	if err != nil {
		return fmt.Errorf("failed to list files from git ref %s: %w", breakingBase, err)
	}
	gitFilesList := strings.Split(strings.TrimSpace(out), "\n")
	var gitFiles []string
	for _, f := range gitFilesList {
		if strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") {
			// We only want files that are under rootPath if we didn't filter by ls-tree (we did).
			// ls-tree returns full path from root.
			// filepath.Walk uses relative path from where we ran it, or absolute if rootPath is abs.
			// ExtractAPI expects paths that match what loader expects.
			// gitLoader expects path relative to root (for git show).
			// localLoader expects path relative to CWD.
			// This mismatch is fine as long as keys in API map (package.func) align.
			gitFiles = append(gitFiles, f)
		}
	}

	// 4. Extract Base API
	gitLoader := func(path string) ([]byte, error) {
		// git show <base>:<path>
		// git show expects path relative to repo root.
		out, err := gitClient.Run(repoRoot, "show", fmt.Sprintf("%s:%s", breakingBase, path))
		if err != nil {
			return nil, err
		}
		return []byte(out), nil
	}

	baseAPI, err := breaking.ExtractAPI(gitFiles, gitLoader)
	if err != nil {
		return fmt.Errorf("failed to extract base API: %w", err)
	}

	// 5. Compare
	changes := breaking.Compare(baseAPI, localAPI)

	// 6. Report
	if breakingJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(changes)
	}

	if len(changes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No changes in public API detected.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TYPE\tIDENTIFIER\tMESSAGE")
	fmt.Fprintln(w, "----\t----------\t-------")

	hasBreaking := false
	for _, c := range changes {
		icon := "ℹ️"
		if c.Type == breaking.ChangeRemoved || c.Type == breaking.ChangeChanged {
			icon = "🚨"
			hasBreaking = true
		} else if c.Type == breaking.ChangeAdded {
			icon = "✨"
		}

		// Truncate message (signature) if too long
		msg := strings.ReplaceAll(c.Message, "\n", " ")
		if len(msg) > 80 {
			msg = msg[:77] + "..."
		}

		fmt.Fprintf(w, "%s %s\t%s\t%s\n", icon, c.Type, c.Identifier, msg)
	}
	w.Flush()

	if breakingFail && hasBreaking {
		return fmt.Errorf("breaking changes detected")
	}

	return nil
}
