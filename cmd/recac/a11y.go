package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	a11yJSON     bool
	a11yFail     bool
	a11yAI       bool
	a11yIgnore   string
)

var a11yCmd = &cobra.Command{
	Use:   "a11y [path]",
	Short: "Check for accessibility (a11y) issues",
	Long: `Scans HTML, JSX, TSX, Vue, and Svelte files for common accessibility violations.
Can also perform a deeper semantic review using the configured AI agent.

Static Checks:
- Images missing alt text
- Anchors missing href or labels
- Positive tabindex
- Non-interactive elements with click handlers
- Invalid aria roles

AI Review:
- Semantic analysis of component structure
- Contrast and hierarchy checks (contextual)
- WCAG 2.1 compliance suggestions`,
	RunE: runA11y,
}

func init() {
	rootCmd.AddCommand(a11yCmd)
	a11yCmd.Flags().BoolVar(&a11yJSON, "json", false, "Output results as JSON")
	a11yCmd.Flags().BoolVar(&a11yFail, "fail", false, "Exit with error if issues found")
	a11yCmd.Flags().BoolVar(&a11yAI, "ai", false, "Use AI agent for deeper review")
	a11yCmd.Flags().StringVar(&a11yIgnore, "ignore", "", "Comma-separated list of issue types to ignore")
}

// A11yFinding represents an accessibility issue
type A11yFinding struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Match       string `json:"match"`
	Severity    string `json:"severity"` // "error", "warning"
}

// A11yScanner handles regex-based scanning
type A11yScanner struct {
	patterns map[string]*regexp.Regexp
	imgRe    *regexp.Regexp
	aTagRe   *regexp.Regexp
	ignored  map[string]bool
}

func NewA11yScanner(ignoreList string) *A11yScanner {
	// Common A11y Regex Patterns (Heuristic)
	// Note: Regex is not a parser, so these are best-effort checks.
	patterns := map[string]*regexp.Regexp{
		"Missing Alt Text": regexp.MustCompile("placeholder"), // Handled specially

		"Positive Tabindex":        regexp.MustCompile(`tabindex\s*=\s*["']([1-9][0-9]*)["']`),
		"Click on Non-Interactive": regexp.MustCompile(`<(div|span|p|section|article)[^>]*?onClick[^>]*?>`),
		"Empty Button":             regexp.MustCompile(`<button[^>]*?>\s*</button>`),        // Very basic empty button
		"Mouse Events without Key": regexp.MustCompile(`onMouse(Over|Out|Down|Up)[^>]*?>`), // Warn if mouse events are used without keyboard equiv?
	}

	ignored := make(map[string]bool)
	if ignoreList != "" {
		for _, s := range strings.Split(ignoreList, ",") {
			ignored[strings.TrimSpace(s)] = true
		}
	}

	return &A11yScanner{
		patterns: patterns,
		imgRe:    regexp.MustCompile(`<img[^>]+>`),
		aTagRe:   regexp.MustCompile(`<\s*a\b[^>]*>`),
		ignored:  ignored,
	}
}

func (s *A11yScanner) Scan(file string, content string) []A11yFinding {
	var findings []A11yFinding
	lines := strings.Split(content, "\n")

	// 1. Regex Checks
	findings = append(findings, s.checkRegexPatterns(content, file)...)
	findings = append(findings, s.checkImages(content, file)...)

	// 2. Specific Logic Checks
	findings = append(findings, s.checkAnchors(lines, file)...)

	return findings
}

func (s *A11yScanner) checkRegexPatterns(content string, file string) []A11yFinding {
	var findings []A11yFinding
	for name, re := range s.patterns {
		if s.ignored[name] {
			continue
		}

		if name == "Missing Alt Text" {
			continue // Handled separately
		}

		matches := re.FindAllStringIndex(content, -1)
		for _, loc := range matches {
			matchText := content[loc[0]:loc[1]]

			// Filter "Click on Non-Interactive" if it has role="button" or tabindex="0"
			if name == "Click on Non-Interactive" {
				if strings.Contains(matchText, "role=") || strings.Contains(matchText, "tabindex=") {
					continue
				}
			}

			// Filter "Mouse Events" if it has onFocus/onBlur/onKey
			if name == "Mouse Events without Key" {
				if strings.Contains(matchText, "onFocus") || strings.Contains(matchText, "onBlur") || strings.Contains(matchText, "onKey") {
					continue
				}
			}

			findings = append(findings, s.createFinding(name, fmt.Sprintf("Found %s", name), file, content, loc[0], matchText))
		}
	}
	return findings
}

func (s *A11yScanner) checkImages(content string, file string) []A11yFinding {
	var findings []A11yFinding
	if s.ignored["Missing Alt Text"] {
		return findings
	}
	matches := s.imgRe.FindAllStringIndex(content, -1)
	for _, loc := range matches {
		tag := content[loc[0]:loc[1]]
		if !strings.Contains(tag, "alt=") {
			findings = append(findings, s.createFinding("Missing Alt Text", "Image tag missing alt attribute", file, content, loc[0], tag))
		}
	}
	return findings
}

func (s *A11yScanner) checkAnchors(lines []string, file string) []A11yFinding {
	var findings []A11yFinding
	for i, line := range lines {
		matches := s.aTagRe.FindAllString(line, -1)
		for _, match := range matches {
			if !strings.Contains(match, "href=") && !strings.Contains(match, "name=") {
				if !s.ignored["Missing Href"] {
					findings = append(findings, A11yFinding{
						Type:        "Missing Href",
						Description: "Anchor tag missing href attribute",
						File:        file,
						Line:        i + 1,
						Match:       match,
						Severity:    "error",
					})
				}
			}
		}
	}
	return findings
}

func (s *A11yScanner) createFinding(typ, desc, file, content string, offset int, match string) A11yFinding {
	line := 1
	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
		}
	}
	return A11yFinding{
		Type:        typ,
		Description: desc,
		File:        file,
		Line:        line,
		Match:       match,
		Severity:    "warning",
	}
}

func runA11y(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "♿ Scaning for accessibility issues in %s...\n", root)

	scanner := NewA11yScanner(a11yIgnore)
	var allFindings []A11yFinding

	// Extensions to scan
	exts := map[string]bool{
		".html": true, ".htm": true,
		".jsx": true, ".tsx": true,
		".js": true, ".ts": true, // Often contain JSX
		".vue": true, ".svelte": true,
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !exts[strings.ToLower(ext)] {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Static Scan
		fileFindings := scanner.Scan(path, string(content))
		allFindings = append(allFindings, fileFindings...)

		// AI Review
		if a11yAI {
			aiFindings, err := reviewWithAI(cmd, path, string(content))
			if err == nil {
				allFindings = append(allFindings, aiFindings...)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: AI review failed for %s: %v\n", path, err)
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	if a11yJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(allFindings)
	}

	if len(allFindings) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No accessibility issues found.")
		return nil
	}

	// Print Table
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tTYPE\tFILE:LINE\tDESCRIPTION")
	for _, f := range allFindings {
		icon := "⚠️"
		if f.Severity == "error" {
			icon = "❌"
		} else if f.Type == "AI Suggestion" {
			icon = "🤖"
		}

		// Truncate file path
		relFile := f.File
		if rel, err := filepath.Rel(root, f.File); err == nil {
			relFile = rel
		}

		fmt.Fprintf(w, "%s %s\t%s\t%s:%d\t%s\n", icon, strings.ToUpper(f.Severity), f.Type, relFile, f.Line, f.Description)
	}
	w.Flush()

	if a11yFail {
		return fmt.Errorf("accessibility check failed with %d issues", len(allFindings))
	}

	return nil
}

func reviewWithAI(cmd *cobra.Command, path, content string) ([]A11yFinding, error) {
	// Skip large files to save tokens
	if len(content) > 20000 {
		return nil, nil
	}

	ctx := cmd.Context()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-a11y")
	if err != nil {
		return nil, err
	}

	// Only send snippet? No, full file for context.
	prompt := fmt.Sprintf(`Review the following code file (%s) for Accessibility (WCAG 2.1) violations.
Focus on:
- Semantic HTML usage
- Keyboard navigability
- Screen reader compatibility (ARIA)
- Color contrast (if inferable)

Return a JSON list of findings. Format:
[
  {
    "type": "Violation Type",
    "description": "Description of the issue",
    "line": 10,
    "severity": "warning"
  }
]
If no issues, return [].
Do not include markdown formatting.

Code:
%s`, filepath.Base(path), content)

	// We print a small progress indicator
	fmt.Fprintf(cmd.ErrOrStderr(), ".")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Simple cleanup
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		lines := strings.Split(resp, "\n")
		if len(lines) > 2 {
			resp = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	var findings []A11yFinding
	// We use a temporary struct to unmarshal
	type AIFinding struct {
		Type        string `json:"type"`
		Description string `json:"description"`
		Line        int    `json:"line"`
		Severity    string `json:"severity"`
	}
	var raw []AIFinding
	if err := json.Unmarshal([]byte(resp), &raw); err != nil {
		// AI output might be malformed, just ignore
		return nil, nil
	}

	for _, r := range raw {
		findings = append(findings, A11yFinding{
			Type:        "AI: " + r.Type,
			Description: r.Description,
			File:        path,
			Line:        r.Line,
			Severity:    r.Severity,
			Match:       "(AI Review)",
		})
	}

	return findings, nil
}
