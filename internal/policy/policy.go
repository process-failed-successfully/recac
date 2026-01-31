package policy

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type RuleType string

const (
	RuleBannedImport  RuleType = "banned_import"
	RuleFileSize      RuleType = "file_size"
	RuleBannedContent RuleType = "banned_content"
)

type Rule struct {
	Type     RuleType `yaml:"type"`
	Pattern  string   `yaml:"pattern,omitempty"`
	MaxLines int      `yaml:"max_lines,omitempty"`
	Message  string   `yaml:"message,omitempty"`

	// Internal use
	compiledRegex *regexp.Regexp
}

type Policy struct {
	Rules []Rule `yaml:"rules"`
}

type Violation struct {
	File    string
	Line    int
	Message string
	Rule    Rule
}

func LoadPolicy(path string) (*Policy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}
	var p Policy
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("failed to parse policy file: %w", err)
	}

	// Pre-compile regexes
	for i := range p.Rules {
		if p.Rules[i].Pattern != "" {
			re, err := regexp.Compile(p.Rules[i].Pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regex in rule %d (%s): %w", i, p.Rules[i].Type, err)
			}
			p.Rules[i].compiledRegex = re
		}
	}

	return &p, nil
}

func (p *Policy) Check(root string) ([]Violation, error) {
	var violations []Violation

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "vendor" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Run rules
		for _, rule := range p.Rules {
			vs, err := checkRule(path, rule)
			if err != nil {
				// Log error but continue check
				continue
			}
			violations = append(violations, vs...)
		}
		return nil
	})

	return violations, err
}

func checkRule(path string, rule Rule) ([]Violation, error) {
	switch rule.Type {
	case RuleFileSize:
		return checkFileSize(path, rule)
	case RuleBannedContent:
		return checkBannedContent(path, rule)
	case RuleBannedImport:
		if strings.HasSuffix(path, ".go") {
			return checkBannedImport(path, rule)
		}
	}
	return nil, nil
}

func checkFileSize(path string, rule Rule) ([]Violation, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lines := 0
	for scanner.Scan() {
		lines++
	}

	if lines > rule.MaxLines {
		msg := rule.Message
		if msg == "" {
			msg = fmt.Sprintf("File exceeds %d lines (actual: %d)", rule.MaxLines, lines)
		}
		return []Violation{{
			File:    path,
			Line:    0,
			Message: msg,
			Rule:    rule,
		}}, nil
	}
	return nil, nil
}

func checkBannedContent(path string, rule Rule) ([]Violation, error) {
	if rule.compiledRegex == nil {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var violations []Violation
	lineNum := 1
	for scanner.Scan() {
		line := scanner.Text()
		if rule.compiledRegex.MatchString(line) {
			msg := rule.Message
			if msg == "" {
				msg = fmt.Sprintf("Found banned content: %s", rule.Pattern)
			}
			violations = append(violations, Violation{
				File:    path,
				Line:    lineNum,
				Message: msg,
				Rule:    rule,
			})
		}
		lineNum++
	}
	return violations, nil
}

func checkBannedImport(path string, rule Rule) ([]Violation, error) {
	if rule.compiledRegex == nil {
		return nil, nil
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var violations []Violation
	for _, imp := range node.Imports {
		if imp.Path == nil {
			continue
		}
		// imp.Path.Value is quoted, e.g. "fmt"
		impPath := strings.Trim(imp.Path.Value, "\"")

		if rule.compiledRegex.MatchString(impPath) {
			msg := rule.Message
			if msg == "" {
				msg = fmt.Sprintf("Banned import: %s", impPath)
			}
			violations = append(violations, Violation{
				File:    path,
				Line:    fset.Position(imp.Pos()).Line,
				Message: msg,
				Rule:    rule,
			})
		}
	}
	return violations, nil
}
