package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	archConfigPath string
	archGenerate   bool
	archFail       bool
)

type ArchConfig struct {
	Layers map[string]string `yaml:"layers"`
	Rules  []ArchRule        `yaml:"rules"`
}

type ArchRule struct {
	From  string   `yaml:"from"`
	Allow []string `yaml:"allow"`
}

var archCmd = &cobra.Command{
	Use:   "arch",
	Short: "Check architectural rules",
	Long: `Enforce architectural boundaries by checking dependency rules.
Define layers (regex) and rules (allowed dependencies) in a YAML config file.

Example Config:
layers:
  domain: "internal/domain/.*"
  app: "internal/app/.*"
rules:
  - from: "domain"
    allow: [] # Domain cannot import other layers
  - from: "app"
    allow: ["domain"]
`,
	RunE: runArch,
}

func init() {
	rootCmd.AddCommand(archCmd)
	archCmd.Flags().StringVar(&archConfigPath, "config", "", "Path to architecture config file")
	archCmd.Flags().BoolVar(&archGenerate, "generate", false, "Generate a default config file")
	archCmd.Flags().BoolVar(&archFail, "fail", true, "Exit with error code if violations found")
}

func runArch(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	// Handle Generate
	if archGenerate {
		return generateDefaultArchConfig(cwd)
	}

	// 1. Load Config
	config, err := loadArchConfig(archConfigPath, cwd)
	if err != nil {
		return fmt.Errorf("failed to load arch config: %w", err)
	}

	// 2. Compile Regexes
	layerRegexps := make(map[string]*regexp.Regexp)
	for name, pattern := range config.Layers {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("invalid regex for layer '%s': %w", name, err)
		}
		layerRegexps[name] = re
	}

	// 3. Analyze Dependencies
	// Reuse logic from map.go
	moduleName, err := getModuleName(cwd)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not read go.mod: %v\n", err)
		moduleName = "unknown"
	}

	deps, err := analyzeDependencies(cwd, moduleName, nil, false) // false = ignore stdlib
	if err != nil {
		return fmt.Errorf("dependency analysis failed: %w", err)
	}

	// 4. Check Violations
	violations := checkViolations(deps, config, layerRegexps)

	// 5. Report
	if len(violations) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ Architecture check passed! No violations found.")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "❌ Found %d architecture violations:\n", len(violations))
	for _, v := range violations {
		fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", v)
	}

	if archFail {
		return fmt.Errorf("architecture check failed")
	}

	return nil
}

func loadArchConfig(path string, cwd string) (*ArchConfig, error) {
	candidates := []string{path, ".recac-arch.yaml", "arch.yaml", ".recac/arch.yaml"}
	var configPath string

	for _, p := range candidates {
		if p == "" {
			continue
		}

		fullPath := p
		if !filepath.IsAbs(p) {
			fullPath = filepath.Join(cwd, p)
		}

		// If path was provided explicitly (first candidate), strict check
		if p == path {
			if _, err := os.Stat(fullPath); err == nil {
				configPath = fullPath
				break
			} else {
				return nil, err
			}
		} else {
			// Check default locations
			if _, err := os.Stat(fullPath); err == nil {
				configPath = fullPath
				break
			}
		}
	}

	if configPath == "" {
		return nil, fmt.Errorf("no architecture config found. Run with --generate to create one")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config ArchConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func generateDefaultArchConfig(cwd string) error {
	defaultConfig := `layers:
  # Adjust regex patterns to match your project structure
  domain: "internal/domain/.*"
  application: "internal/application/.*"
  infrastructure: "internal/infrastructure/.*"
  api: "internal/api/.*"
  cmd: "cmd/.*"

rules:
  # Define allowed dependencies (directed graph)
  - from: "domain"
    allow: [] # Domain should be independent
  - from: "application"
    allow: ["domain"]
  - from: "infrastructure"
    allow: ["domain", "application"]
  - from: "api"
    allow: ["application", "domain"]
  - from: "cmd"
    allow: ["api", "infrastructure", "application"]
`
	target := filepath.Join(cwd, ".recac-arch.yaml")
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists", target)
	}

	if err := os.WriteFile(target, []byte(defaultConfig), 0644); err != nil {
		return err
	}

	fmt.Printf("✅ Created default architecture config: %s\n", target)
	return nil
}

func checkViolations(deps DepMap, config *ArchConfig, regexps map[string]*regexp.Regexp) []string {
	var violations []string

	// Deterministic layer matching
	var layerNames []string
	for name := range regexps {
		layerNames = append(layerNames, name)
	}
	sort.Strings(layerNames)

	// Helper to identify layer
	getLayer := func(pkg string) string {
		for _, name := range layerNames {
			re := regexps[name]
			if re.MatchString(pkg) {
				return name
			}
		}
		return "" // No layer assigned (ignored)
	}

	// Build allowed map for O(1) lookup
	// Layer -> Set of Allowed Layers
	allowed := make(map[string]map[string]bool)
	for _, rule := range config.Rules {
		if allowed[rule.From] == nil {
			allowed[rule.From] = make(map[string]bool)
		}
		// Always allow self
		allowed[rule.From][rule.From] = true
		for _, allow := range rule.Allow {
			allowed[rule.From][allow] = true
		}
	}

	for srcPkg, targets := range deps {
		srcLayer := getLayer(srcPkg)
		if srcLayer == "" {
			continue // Source package not in any monitored layer
		}

		// Check if we have rules for this layer
		allowedLayers, hasRule := allowed[srcLayer]
		if !hasRule {
			// If no rule is defined for a layer, do we allow everything or nothing?
			// Usually, if listed in layers but not in rules, it implies no restrictions?
			// Or should we strict defaults?
			// Let's assume strict: if listed in layers, must have rule.
			// But for now, let's assume if not in rules, it's free.
			// Actually, typical ArchUnit style: if rule exists, enforce it.
			continue
		}

		for _, tgtPkg := range targets {
			tgtLayer := getLayer(tgtPkg)
			if tgtLayer == "" {
				continue // Target not in any monitored layer (e.g. utils, or external)
			}

			if !allowedLayers[tgtLayer] {
				violations = append(violations, fmt.Sprintf("%s (%s) imports %s (%s)", srcPkg, srcLayer, tgtPkg, tgtLayer))
			}
		}
	}

	return violations
}
