package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var (
	ownersGenerate bool
)

var ownersCmd = &cobra.Command{
	Use:   "owners [path]",
	Short: "Identify code owners or generate a CODEOWNERS file",
	Long: `Identify the owner of a file or directory based on CODEOWNERS rules or git history.
Can also generate a draft CODEOWNERS file by analyzing contributor history.`,
	RunE: runOwners,
}

func init() {
	rootCmd.AddCommand(ownersCmd)
	ownersCmd.Flags().BoolVar(&ownersGenerate, "generate", false, "Generate a draft CODEOWNERS file based on git history")
}

func runOwners(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := gitClientFactory()
	if !client.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// Find repo root for correct CODEOWNERS resolution
	root, err := findRepoRoot(cwd)
	if err != nil {
		// Fallback to cwd if finding root fails (e.g. testing without git)
		root = cwd
	}

	if ownersGenerate {
		return generateOwners(cmd, root)
	}

	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	// Determine absolute target path
	absTarget := targetPath
	if !filepath.IsAbs(targetPath) {
		absTarget = filepath.Join(cwd, targetPath)
	}

	// Normalize target path relative to repo root for matching
	relPath, err := filepath.Rel(root, absTarget)
	if err != nil {
		return err
	}

	// 1. Try CODEOWNERS
	owners, source, err := resolveCodeOwners(root, relPath)
	if err == nil && len(owners) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "Owners of %s (from %s):\n", relPath, source)
		for _, o := range owners {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s\n", o)
		}
		return nil
	}

	// 2. Fallback to Git History
	fmt.Fprintf(cmd.OutOrStdout(), "No CODEOWNERS rule found for '%s'. Analyzing git history...\n", relPath)

	// git log --pretty=format:%an <%ae> -- <path>
	logs, err := client.Log(cwd, "--pretty=format:%an <%ae>", "--", targetPath)
	if err != nil {
		return fmt.Errorf("git log failed: %w", err)
	}

	if len(logs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No git history found for this path.")
		return nil
	}

	// Count frequency
	counts := make(map[string]int)
	for _, line := range logs {
		line = strings.TrimSpace(line)
		if line != "" {
			counts[line]++
		}
	}

	// Sort
	type entry struct {
		Name  string
		Count int
	}
	var sorted []entry
	for name, count := range counts {
		sorted = append(sorted, entry{Name: name, Count: count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Count > sorted[j].Count
	})

	fmt.Fprintf(cmd.OutOrStdout(), "Top contributors for %s:\n", targetPath)
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		e := sorted[i]
		percentage := float64(e.Count) / float64(len(logs)) * 100
		fmt.Fprintf(cmd.OutOrStdout(), "- %s (%d commits, %.1f%%)\n", e.Name, e.Count, percentage)
	}

	return nil
}

// resolveCodeOwners attempts to find the owner from CODEOWNERS files
func resolveCodeOwners(root, targetRelPath string) ([]string, string, error) {
	// Standard locations: CODEOWNERS, .github/CODEOWNERS, docs/CODEOWNERS
	locations := []string{"CODEOWNERS", ".github/CODEOWNERS", "docs/CODEOWNERS"}

	var lines []string
	var loadedFile string

	for _, loc := range locations {
		path := filepath.Join(root, loc)
		if l, err := utils.ReadLines(path); err == nil {
			lines = l
			loadedFile = loc
			break
		}
	}

	if loadedFile == "" {
		return nil, "", fmt.Errorf("no CODEOWNERS file found")
	}

	// Parse lines (last match wins)
	var lastMatch []string

	// Ensure target uses forward slash for matching
	targetSlash := filepath.ToSlash(targetRelPath)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		pattern := parts[0]
		owners := parts[1:]

		// Match logic
		matched, err := matchCodeOwnerPattern(pattern, targetSlash)
		if err != nil {
			continue
		}
		if matched {
			lastMatch = owners
		}
	}

	if len(lastMatch) > 0 {
		return lastMatch, loadedFile, nil
	}

	return nil, loadedFile, nil
}

func findRepoRoot(cwd string) (string, error) {
	cmd := execCommand("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// matchCodeOwnerPattern checks if a file path matches a CODEOWNERS pattern
func matchCodeOwnerPattern(pattern, file string) (bool, error) {
	isAnchored := strings.HasPrefix(pattern, "/")
	if isAnchored {
		pattern = pattern[1:]
	}

	// 1. If pattern ends with /, it matches a directory
	if strings.HasSuffix(pattern, "/") {
		cleanPattern := strings.TrimSuffix(pattern, "/")
		if file == cleanPattern || strings.HasPrefix(file, cleanPattern+"/") {
			return true, nil
		}
		// If anchored, we are done checking directories
		if isAnchored {
			return false, nil
		}
		// If not anchored, it could match subdirectories (e.g. "docs/" matches "src/docs/")
		// For MVP, we'll skip deep recursive check unless it matches exact prefix
		return false, nil
	}

	// 2. Standard glob matching
	if matched, _ := filepath.Match(pattern, file); matched {
		return true, nil
	}

	// 3. If not anchored, try matching against basename for simple patterns
	if !isAnchored && !strings.Contains(pattern, "/") {
		// Pattern "*.js" matches "src/foo.js"
		if matched, _ := filepath.Match(pattern, filepath.Base(file)); matched {
			return true, nil
		}
	}

	// 4. Directory prefix fallback (e.g. "internal" matches "internal/foo.go")
	if !strings.Contains(pattern, "*") {
		if strings.HasPrefix(file, pattern+"/") {
			return true, nil
		}
	}

	return false, nil
}

func generateOwners(cmd *cobra.Command, root string) error {
	client := gitClientFactory()
	ignoreMap := DefaultIgnoreMap() // from shared_utils.go

	fmt.Fprintln(cmd.OutOrStdout(), "Analyzing repository history to generate CODEOWNERS...")

	// Walk the repo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden and ignored
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if ignoreMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Since walking every file and running git log is too slow,
	// let's just list all files in git and parse?
	// Or iterate top-level directories.

	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}

	var outputLines []string
	outputLines = append(outputLines, "# Auto-generated CODEOWNERS based on git history")
	outputLines = append(outputLines, "# This is a draft. Please review and adjust.\n")

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || ignoreMap[name] {
			continue
		}

		// We analyze this entry (file or dir)
		// git log -n 100 --pretty=format:%an -- <name>
		logs, err := client.Log(root, "-n", "100", "--pretty=format:%ae", "--", name)
		if err != nil {
			continue
		}

		if len(logs) == 0 {
			continue
		}

		counts := make(map[string]int)
		for _, l := range logs {
			if strings.TrimSpace(l) != "" {
				counts[strings.TrimSpace(l)]++
			}
		}

		// Find winner
		var bestAuthor string
		var maxCount int
		for author, count := range counts {
			if count > maxCount {
				maxCount = count
				bestAuthor = author
			}
		}

		if bestAuthor != "" {
			// If it's a directory, append /
			pattern := name
			if entry.IsDir() {
				pattern = name + "/"
			}
			outputLines = append(outputLines, fmt.Sprintf("%-30s %s", pattern, bestAuthor))
		}
	}

	// Print result
	fmt.Fprintln(cmd.OutOrStdout(), strings.Join(outputLines, "\n"))
	return nil
}
