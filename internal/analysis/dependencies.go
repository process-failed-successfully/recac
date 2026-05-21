package analysis

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"

	"recac/internal/utils"
)

// DepMap maps a source package path to a list of imported package paths.
type DepMap map[string][]string

// DependencyOptions configures the analysis behavior.
type DependencyOptions struct {
	Root           string
	ModuleName     string
	IgnorePatterns []string
	ShowStdLib     bool
}

// AnalyzeDependencies scans the Go codebase and builds a dependency graph.
func AnalyzeDependencies(opts DependencyOptions) (DepMap, error) {
	deps := make(DepMap)
	fset := token.NewFileSet()

	// Pre-compile ignore regexes
	var ignoreRegexps []*regexp.Regexp
	for _, pattern := range opts.IgnorePatterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid ignore pattern '%s': %w", pattern, err)
		}
		ignoreRegexps = append(ignoreRegexps, re)
	}

	// Cache for regex checks
	checkedImports := make(map[string]bool)

	// Determine module name if not provided
	moduleName := opts.ModuleName
	if moduleName == "" {
		mn, err := GetModuleName(opts.Root)
		if err != nil {
			// Fallback: use directory name as last resort? No, better warn.
			// But for now, let's just proceed with empty moduleName (might break relative pkg calc).
		} else {
			moduleName = mn
		}
	}

	err := filepath.WalkDir(opts.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".recac" {
				return filepath.SkipDir
			}
			// Check ignore patterns for directory
			for _, re := range ignoreRegexps {
				if re.MatchString(path) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Calculate package path relative to module
		dir := filepath.Dir(path)
		relDir, _ := filepath.Rel(opts.Root, dir)
		if relDir == "." {
			relDir = ""
		}

		pkgPath := moduleName
		if relDir != "" {
			pkgPath = filepath.Join(moduleName, relDir)
		}
		// Windows fix
		pkgPath = filepath.ToSlash(pkgPath)

		// Check ignore patterns for source package
		for _, re := range ignoreRegexps {
			if re.MatchString(pkgPath) {
				return nil
			}
		}

		// Parse file imports
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil // Skip unparseable files
		}

		for _, imp := range f.Imports {
			target := strings.Trim(imp.Path.Value, "\"")

			// Check ignore patterns for target package
			ignored := false
			if v, ok := checkedImports[target]; ok {
				ignored = v
			} else {
				for _, re := range ignoreRegexps {
					if re.MatchString(target) {
						ignored = true
						break
					}
				}
				checkedImports[target] = ignored
			}

			if ignored {
				continue
			}

			if !opts.ShowStdLib && !strings.Contains(target, ".") {
				// Rough heuristic for stdlib: no dot (usually)
				// But internal module packages might also lack dots if module name lacks dot (unlikely).
				// Better heuristic: if not prefixed with moduleName AND no dot?
				// stdlib packages: "fmt", "os", "net/http".
				// module packages: "github.com/org/repo/pkg".
				// So if it doesn't have a dot (like "fmt") OR starts with "encoding/", it's stdlib.
				// However, "net/http" has a dot.
				// The map.go logic was: `!strings.Contains(target, ".")`
				// AND `!strings.HasPrefix(target, moduleName)`.
				// Wait, `net/http` contains dot? No, slash. "net.http"? No.
				// "github.com" has dot.
				// So stdlib usually has NO dot in the *domain part*.
				// Actually standard library packages do not have a dot. "math/rand" -> no dot.
				// "golang.org/x/..." -> has dot.
				// So `!strings.Contains(target, ".")` is a decent heuristic for stdlib.
				if !strings.HasPrefix(target, moduleName) && !strings.Contains(target, ".") {
					continue
				}
			}

			// Add dependency
			// Avoid duplicates
			found := false
			for _, existing := range deps[pkgPath] {
				if existing == target {
					found = true
					break
				}
			}
			if !found {
				deps[pkgPath] = append(deps[pkgPath], target)
			}
		}

		return nil
	})

	return deps, err
}

func GetModuleName(root string) (string, error) {
	goModPath := filepath.Join(root, "go.mod")
	lines, err := utils.ReadLines(goModPath)
	if err != nil {
		return "", err
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("module declaration not found")
}
