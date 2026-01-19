package main

import (
	"bufio"
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var mapCmd = &cobra.Command{
	Use:   "map [path]",
	Short: "Visualize code architecture",
	Long: `Generates a dependency graph of the Go packages in the project.
Can output in Mermaid (default) or DOT format.
Use --explain to have the AI analyze the architecture.`,
	RunE: runMap,
}

func init() {
	rootCmd.AddCommand(mapCmd)
	initMapFlags(mapCmd)
}

func initMapFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", "", "Output file path")
	cmd.Flags().StringP("format", "f", "mermaid", "Output format (mermaid, dot)")
	cmd.Flags().Bool("explain", false, "Use AI to explain the architecture")
	cmd.Flags().StringSliceP("ignore", "i", []string{}, "Ignore packages matching regex")
	cmd.Flags().String("focus", "", "Focus on a specific package (substring)")
	cmd.Flags().Bool("stdlib", false, "Include standard library dependencies")
}

func runMap(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// Read flags
	mapOutput, _ := cmd.Flags().GetString("output")
	mapFormat, _ := cmd.Flags().GetString("format")
	mapExplain, _ := cmd.Flags().GetBool("explain")
	mapIgnore, _ := cmd.Flags().GetStringSlice("ignore")
	// Clean up pflag artifacts
	var cleanIgnore []string
	for _, p := range mapIgnore {
		if p != "[]" {
			cleanIgnore = append(cleanIgnore, p)
		}
	}
	mapIgnore = cleanIgnore

	// fmt.Fprintf(cmd.ErrOrStderr(), "DEBUG: ignore=%v\n", mapIgnore)

	mapFocus, _ := cmd.Flags().GetString("focus")
	mapShowStdLib, _ := cmd.Flags().GetBool("stdlib")

	// 1. Determine Module Name
	moduleName, err := getModuleName(root)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read go.mod: %v\n", err)
	}

	// Pre-compile ignore regexes
	var ignoreRegexps []*regexp.Regexp
	for _, pattern := range mapIgnore {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid ignore pattern '%s': %w", pattern, err)
		}
		ignoreRegexps = append(ignoreRegexps, re)
	}

	// 2. Analyze Dependencies
	deps, err := analyzeDependencies(root, moduleName, ignoreRegexps, mapShowStdLib)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// 4. Generate Output
	var output string
	switch strings.ToLower(mapFormat) {
	case "dot":
		output = generateDOT(deps, moduleName, mapFocus)
	case "mermaid":
		output = generateMermaidMap(deps, moduleName, mapFocus)
	default:
		return fmt.Errorf("unknown format: %s", mapFormat)
	}

	// 5. Handle Output
	if mapOutput != "" {
		if err := os.WriteFile(mapOutput, []byte(output), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Graph saved to %s\n", mapOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), output)
	}

	// 6. Explain if requested
	if mapExplain {
		return explainArchitecture(cmd, output)
	}

	return nil
}

func getModuleName(root string) (string, error) {
	goModPath := filepath.Join(root, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("module declaration not found")
}

// map[sourcePackage] -> []targetPackages
type DepMap map[string][]string

func analyzeDependencies(root string, moduleName string, ignoreRegexps []*regexp.Regexp, showStdLib bool) (DepMap, error) {
	deps := make(DepMap)
	fset := token.NewFileSet()

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "vendor" || name == "node_modules" || name == ".recac" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Calculate package path relative to module
		dir := filepath.Dir(path)
		relDir, _ := filepath.Rel(root, dir)
		if relDir == "." {
			relDir = ""
		}

		pkgPath := moduleName
		if relDir != "" {
			pkgPath = filepath.Join(moduleName, relDir)
		}
		// Windows fix
		pkgPath = strings.ReplaceAll(pkgPath, "\\", "/")

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
			for _, re := range ignoreRegexps {
				if re.MatchString(target) {
					ignored = true
					break
				}
			}
			if ignored {
				continue
			}

			if !showStdLib && !strings.Contains(target, ".") {
				// Rough heuristic for stdlib
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

func generateMermaidMap(deps DepMap, moduleName string, focus string) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort keys for deterministic output
	var keys []string
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, src := range keys {
		// Filter focus
		if focus != "" && !strings.Contains(src, focus) {
			continue
		}

		safeSrc := sanitizeID(src)

		targets := deps[src]
		sort.Strings(targets)

		for _, tgt := range targets {
			safeTgt := sanitizeID(tgt)

			// If moduleName is known, highlight internal vs external
			if strings.HasPrefix(tgt, moduleName) {
				sb.WriteString(fmt.Sprintf("    %s --> %s\n", safeSrc, safeTgt))
			} else {
				// External dependency style
				sb.WriteString(fmt.Sprintf("    %s -.-> %s\n", safeSrc, safeTgt))
			}
		}
	}

	return sb.String()
}

func generateDOT(deps DepMap, moduleName string, focus string) string {
	var sb strings.Builder
	sb.WriteString("digraph G {\n")
	sb.WriteString("  rankdir=LR;\n")
	sb.WriteString("  node [shape=box, style=filled, color=\"#dddddd\"];\n")

	var keys []string
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, src := range keys {
		// Filter focus
		if focus != "" && !strings.Contains(src, focus) {
			continue
		}

		targets := deps[src]
		sort.Strings(targets)
		for _, tgt := range targets {
			sb.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", src, tgt))
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

func sanitizeID(id string) string {
	id = strings.ReplaceAll(id, "/", "_")
	id = strings.ReplaceAll(id, "-", "_")
	id = strings.ReplaceAll(id, ".", "_")
	return id
}

func explainArchitecture(cmd *cobra.Command, graphStr string) error {
	ctx := context.Background()
	cwd, _ := os.Getwd()

	// Create agent
	ag, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-map")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Analyze the following Go dependency graph (Mermaid/DOT) and describe the software architecture.
Identify:
1. Core components and their responsibilities.
2. Potential architectural bottlenecks or circular dependencies (if any).
3. The overall architectural pattern (e.g., Layered, Hexagonal, Monolith).

Graph:
'''
%s
'''`, graphStr)

	fmt.Fprintln(cmd.OutOrStdout(), "\n🤖 Analyzing architecture...")

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Println("")

	return err
}
