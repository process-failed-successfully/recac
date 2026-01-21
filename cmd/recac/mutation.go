package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/mutation"
	"strings"

	"github.com/spf13/cobra"
)

var (
	mutationDiff    bool
	mutationVerbose bool
	mutationDryRun  bool
)

var mutationCmd = &cobra.Command{
	Use:   "mutation [package_path]",
	Short: "Run mutation testing on a package",
	Long: `Performs mutation testing by modifying source code (e.g., changing operators) and running tests.
If tests fail, the mutant is "Killed" (Good).
If tests pass, the mutant "Survived" (Bad, indicates missing test coverage).

WARNING: This command copies the package to a temporary directory to run tests safely.
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMutation,
}

func init() {
	rootCmd.AddCommand(mutationCmd)
	mutationCmd.Flags().BoolVar(&mutationDiff, "diff", false, "Show the diff of surviving mutants")
	mutationCmd.Flags().BoolVarP(&mutationVerbose, "verbose", "v", false, "Verbose output")
	mutationCmd.Flags().BoolVar(&mutationDryRun, "dry-run", false, "Generate mutants without running tests")
}

func runMutation(cmd *cobra.Command, args []string) error {
	pkgPath := "."
	if len(args) > 0 {
		pkgPath = args[0]
	}

	// 1. Verify it's a Go package
	if _, err := os.Stat(filepath.Join(pkgPath, "go.mod")); os.IsNotExist(err) {
		// If not root, check if files exist
	}

	absPath, err := filepath.Abs(pkgPath)
	if err != nil {
		return err
	}

	// 2. Setup Temp Directory
	tempDir, err := os.MkdirTemp("", "recac-mutation-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if mutationVerbose {
		fmt.Printf("📂 Created temp workspace: %s\n", tempDir)
	}

	// Copy the package to temp dir
	// We need to copy recursively if it's a module, but ideally we just copy the target package and go.mod
	// Simpler approach: Copy the entire directory structure to preserve imports if relative.
	// For this MVP, let's assume we copy the contents of pkgPath to tempDir.
	// NOTE: If the package relies on internal packages up the tree, this breaks.
	// Robust way: Copy the entire module root? That might be huge.
	// Best effort: Copy contents of pkgPath. If build fails, user needs to run from module root.

	// Let's assume we run ON the module root or we copy enough.
	// To be safe, let's just copy everything in current dir to temp (excluding .git, etc)
	// Actually, `go test` needs dependencies.

	// Fast approach: don't copy everything.
	// Just backup the files we mutate in place? NO. DANGEROUS.

	// Let's rely on `go test` being smart.
	// We will copy the target package files to tempDir.
	// But `go.mod` needs to be there.

	// Let's try to find go.mod
	goModPath := findGoMod(absPath)
	if goModPath == "" {
		return fmt.Errorf("go.mod not found in tree")
	}
	moduleRoot := filepath.Dir(goModPath)

	// We'll copy the WHOLE module to tempDir. This is heavy but safe.
	// We exclude .git, node_modules, etc.
	if mutationVerbose {
		fmt.Println("📦 Copying module to sandbox...")
	}
	if err := copyDir(moduleRoot, tempDir); err != nil {
		return fmt.Errorf("failed to copy project: %w", err)
	}

	// Determine the relative path of the target package inside tempDir
	relPath, _ := filepath.Rel(moduleRoot, absPath)
	targetDir := filepath.Join(tempDir, relPath)

	// 3. Parse and Generate Mutations
	fset := token.NewFileSet()
	mutator := mutation.NewMutator(fset)

	var allMutations []mutation.Mutation
	var files []*ast.File
	var fileNames []string

	// Walk targetDir
	err = filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != targetDir {
			return filepath.SkipDir // Only mutate files in the target package, not subpackages
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Read file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		file, err := parser.ParseFile(fset, path, content, parser.ParseComments)
		if err != nil {
			return err
		}

		files = append(files, file)
		fileNames = append(fileNames, path)

		// Generate mutations
		muts := mutator.GenerateMutations(file, path)
		allMutations = append(allMutations, muts...)
		return nil
	})

	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d possible mutations in %s\n", len(allMutations), pkgPath)

	if mutationDryRun {
		for i, m := range allMutations {
			fmt.Printf("[%d] %s: %s -> %s (line %d)\n", i, m.Type, m.Original, m.Mutated, m.Line)
		}
		return nil
	}

	// 4. Baseline Test Run
	if mutationVerbose {
		fmt.Println("running baseline tests...")
	}
	if err := runGoTest(targetDir); err != nil {
		return fmt.Errorf("baseline tests failed! Fix tests before running mutation testing. Error: %v", err)
	}

	// Create a map for fast file lookup
	fileMap := make(map[string]*ast.File)
	for i, name := range fileNames {
		fileMap[name] = files[i]
	}

	// 5. Execute Mutations
	killed := 0
	survived := 0

	// Progress bar simulation
	total := len(allMutations)

	for i, m := range allMutations {
		// Apply mutation in memory (AST)
		m.Apply()

		// Find the AST file node
		astFile, ok := fileMap[m.File]
		if !ok {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: could not find AST for %s\n", m.File)
			continue
		}

		// Write modified file to disk
		f, err := os.Create(m.File)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error writing file %s: %v\n", m.File, err)
			continue
		}
		printer.Fprint(f, fset, astFile)
		f.Close()

		if mutationVerbose {
			fmt.Printf("[%d/%d] Mutating %s:%d (%s -> %s)... ", i+1, total, filepath.Base(m.File), m.Line, m.Original, m.Mutated)
		}

		// Run tests
		err = runGoTest(targetDir)

		if err != nil {
			// Tests failed -> Mutant Killed!
			killed++
			if mutationVerbose {
				fmt.Println("✅ Killed")
			}
		} else {
			// Tests passed -> Mutant Survived!
			survived++
			if mutationVerbose {
				fmt.Println("❌ Survived")
			}
			// Always print survivors
			fmt.Fprintf(cmd.OutOrStdout(), "❌ Mutant Survived: %s:%d (%s -> %s)\n", filepath.Base(m.File), m.Line, m.Original, m.Mutated)
		}

		// Revert mutation in AST
		m.Revert()

		// Restore file on disk (write back original)
		f, _ = os.Create(m.File)
		printer.Fprint(f, fset, astFile)
		f.Close()
	}

	// 6. Report
	score := 0.0
	if total > 0 {
		score = float64(killed) / float64(total) * 100
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nMutation Testing Results:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Total Mutants: %d\n", total)
	fmt.Fprintf(cmd.OutOrStdout(), "Killed:        %d\n", killed)
	fmt.Fprintf(cmd.OutOrStdout(), "Survived:      %d\n", survived)
	fmt.Fprintf(cmd.OutOrStdout(), "Mutation Score: %.2f%%\n", score)

	if survived > 0 {
		return fmt.Errorf("mutation score below 100%%")
	}

	return nil
}

func findGoMod(path string) string {
	for {
		if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
			return filepath.Join(path, "go.mod")
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func runGoTest(dir string) error {
	// We use "go test ." so it runs only the package in the dir
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = dir
	// We don't want output unless verbose, or maybe just errors?
	// For mutation testing, we just care about exit code.
	return cmd.Run()
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden dirs like .git, .recac
		if info.IsDir() {
			if strings.HasPrefix(info.Name(), ".") && info.Name() != "." {
				return filepath.SkipDir
			}
			if info.Name() == "node_modules" || info.Name() == "vendor" {
				return filepath.SkipDir
			}
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		// Copy file
		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(target)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
