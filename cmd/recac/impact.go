package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var impactCmd = &cobra.Command{
	Use:   "impact [files...]",
	Short: "Analyze the impact of changes",
	Long: `Identifies which packages and tests are affected by changes to specific files.
This command builds a reverse dependency graph to show the ripple effect of changes.

Examples:
  recac impact internal/util/helper.go
  recac impact --git-diff
`,
	RunE: runImpact,
}

var (
	impactJSON         bool
	impactSuggestTests bool
	impactGitDiff      bool
)

func init() {
	rootCmd.AddCommand(impactCmd)
	impactCmd.Flags().BoolVar(&impactJSON, "json", false, "Output results as JSON")
	impactCmd.Flags().BoolVar(&impactSuggestTests, "suggest-tests", false, "Suggest relevant tests to run")
	impactCmd.Flags().BoolVar(&impactGitDiff, "git-diff", false, "Analyze changes in current git diff (unstaged)")
}

type ImpactResult struct {
	AffectedPackages []string `json:"affected_packages"`
	SuggestedTests   []string `json:"suggested_tests,omitempty"`
}

func runImpact(cmd *cobra.Command, args []string) error {
	root := "."

	files := args
	if impactGitDiff {
		diffFiles, err := getGitDiffFiles(false)
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}
		files = append(files, diffFiles...)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files specified (use [files...] or --git-diff)")
	}

	sortedAffected, changedPackages, err := IdentifyImpactedPackages(files, root)
	if err != nil {
		if strings.Contains(err.Error(), "No Go packages found") {
			if impactJSON {
				fmt.Fprintln(cmd.OutOrStdout(), "{}")
				return nil
			}
			fmt.Fprintln(cmd.OutOrStdout(), "No Go packages found for the specified files.")
			return nil
		}
		return err
	}

	// Determine module name for test suggestion path resolution
	moduleName, err := getModuleName(root)
	if err != nil {
		return fmt.Errorf("could not determine module name: %w", err)
	}

	var suggestedTests []string
	if impactSuggestTests {
		for _, pkg := range sortedAffected {
			if strings.HasPrefix(pkg, moduleName) {
				relPath := strings.TrimPrefix(pkg, moduleName)
				relPath = strings.TrimPrefix(relPath, "/")
				if relPath == "" {
					relPath = "."
				}
				fullPath := filepath.Join(root, relPath)
				if hasTests(fullPath) {
					suggestedTests = append(suggestedTests, fmt.Sprintf("go test %s", pkg))
				}
			}
		}
		sort.Strings(suggestedTests)
	}

	res := ImpactResult{
		AffectedPackages: sortedAffected,
		SuggestedTests:   suggestedTests,
	}

	if impactJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	// Text Output
	fmt.Fprintf(cmd.OutOrStdout(), "Changed Packages (%d):\n", len(changedPackages))
	for pkg := range changedPackages {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", pkg)
	}
	fmt.Println()

	fmt.Fprintf(cmd.OutOrStdout(), "Affected Packages (%d):\n", len(sortedAffected))
	for _, pkg := range sortedAffected {
		marker := ""
		if changedPackages[pkg] {
			marker = " (changed)"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s%s\n", pkg, marker)
	}

	if impactSuggestTests && len(suggestedTests) > 0 {
		fmt.Println()
		fmt.Fprintln(cmd.OutOrStdout(), "Suggested Tests:")
		for _, testCmd := range suggestedTests {
			fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", testCmd)
		}
	}

	return nil
}

// IdentifyImpactedPackages analyzes dependencies and returns a list of affected packages.
// It also returns the set of directly changed packages for reference.
func IdentifyImpactedPackages(files []string, root string) ([]string, map[string]bool, error) {
	// 1. Determine Module Name
	moduleName, err := getModuleName(root)
	if err != nil {
		return nil, nil, fmt.Errorf("could not determine module name: %w", err)
	}

	// 2. Map Files to Packages
	changedPackages := make(map[string]bool)
	for _, f := range files {
		pkg, err := fileToPackage(root, moduleName, f)
		if err != nil {
			continue
		}
		if pkg != "" {
			changedPackages[pkg] = true
		}
	}

	if len(changedPackages) == 0 {
		return nil, nil, fmt.Errorf("No Go packages found for the specified files.")
	}

	// 3. Analyze Dependencies (Full Graph)
	deps, err := analyzeDependencies(root, moduleName, nil, false)
	if err != nil {
		return nil, nil, fmt.Errorf("dependency analysis failed: %w", err)
	}

	// 4. Invert Graph (Target -> Sources)
	revDeps := invertDependencies(deps)

	// 5. Find All Dependents (Transitive)
	affected := make(map[string]bool)
	var queue []string
	for pkg := range changedPackages {
		queue = append(queue, pkg)
		affected[pkg] = true // The changed package itself is affected
	}

	visited := make(map[string]bool)
	for _, p := range queue {
		visited[p] = true
	}

	head := 0
	for head < len(queue) {
		current := queue[head]
		head++

		// Find who imports 'current'
		importers := revDeps[current]
		for _, importer := range importers {
			if !visited[importer] {
				visited[importer] = true
				affected[importer] = true
				queue = append(queue, importer)
			}
		}
	}

	// 6. Format Output
	var sortedAffected []string
	for pkg := range affected {
		sortedAffected = append(sortedAffected, pkg)
	}
	sort.Strings(sortedAffected)

	return sortedAffected, changedPackages, nil
}

func fileToPackage(root, moduleName, file string) (string, error) {
	if !strings.HasSuffix(file, ".go") {
		return "", nil
	}

	absFile, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(absRoot, absFile)
	if err != nil {
		return "", err
	}

	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("file is outside module root")
	}

	dir := filepath.Dir(rel)
	if dir == "." {
		return moduleName, nil
	}

	dir = strings.ReplaceAll(dir, "\\", "/")
	return fmt.Sprintf("%s/%s", moduleName, dir), nil
}

func invertDependencies(deps DepMap) map[string][]string {
	rev := make(map[string][]string)
	for src, targets := range deps {
		for _, tgt := range targets {
			rev[tgt] = append(rev[tgt], src)
		}
	}
	return rev
}

func hasTests(dir string) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), "_test.go") {
			return true
		}
	}
	return false
}

func getGitDiffFiles(staged bool) ([]string, error) {
	args := []string{"diff", "--name-only"}
	if staged {
		args = append(args, "--cached")
	}
	cmd := exec.Command("git", args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out.String(), "\n")
	var files []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}
