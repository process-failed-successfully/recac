package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
)

var doctestCmd = &cobra.Command{
	Use:   "doctest [file...]",
	Short: "Verify code examples in documentation",
	Long: `Parses Markdown files, extracts code blocks (go, json, yaml, bash), and validates them.
- Go: Compiles the code (checks for syntax errors).
- JSON/YAML: Validates syntax.
- Bash: Checks syntax (using bash -n).

If no files are provided, defaults to README.md.`,
	RunE: runDoctest,
}

func init() {
	rootCmd.AddCommand(doctestCmd)
}

func runDoctest(cmd *cobra.Command, args []string) error {
	files := args
	if len(files) == 0 {
		files = []string{"README.md"}
	}

	hasErrors := false

	for _, file := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "Checking %s...\n", file)

		if _, err := os.Stat(file); os.IsNotExist(err) {
			fmt.Fprintf(cmd.OutOrStdout(), "  ⚠️ File not found: %s\n", file)
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", file, err)
		}

		failures, err := checkFile(file, content, cmd.OutOrStdout())
		if err != nil {
			return err
		}

		if failures > 0 {
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("doctest failed")
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✅ All checks passed!")
	return nil
}

func checkFile(filename string, content []byte, out io.Writer) (int, error) {
	parser := goldmark.DefaultParser()
	reader := text.NewReader(content)
	doc := parser.Parse(reader)

	failures := 0

	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		if codeBlock, ok := n.(*ast.FencedCodeBlock); ok {
			lang := string(codeBlock.Language(content))

			// Extract code
			var buf bytes.Buffer
			lines := codeBlock.Lines()
			for i := 0; i < lines.Len(); i++ {
				line := lines.At(i)
				buf.Write(line.Value(content))
			}
			code := buf.String()

			// Line number estimation (Goldmark doesn't give direct line numbers easily in simple walk,
			// but we can try to guess or just report the block index if needed.
			// Actually, FencedCodeBlock has Info but not line number directly in all versions.
			// Ideally we report approximate location.)
			// We can verify language.

			if lang == "" {
				return ast.WalkContinue, nil
			}

			err := validateBlock(lang, code)
			if err != nil {
				failures++
				fmt.Fprintf(out, "  ❌ [%s] Failed:\n%s\n", lang, indent(err.Error(), "    "))
			} else {
				// Verbose?
				// fmt.Fprintf(out, "  ✅ [%s] OK\n", lang)
			}
		}
		return ast.WalkContinue, nil
	})

	return failures, err
}

func validateBlock(lang, code string) error {
	switch strings.ToLower(lang) {
	case "json":
		var js interface{}
		return json.Unmarshal([]byte(code), &js)
	case "yaml", "yml":
		var y interface{}
		return yaml.Unmarshal([]byte(code), &y)
	case "go":
		return validateGo(code)
	case "bash", "sh":
		return validateBash(code)
	default:
		// Ignore unknown languages
		return nil
	}
}

func validateGo(code string) error {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "recac-doctest-go")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Check if code has package declaration
	if !strings.Contains(code, "package ") {
		// If snippet, wrap in main
		code = fmt.Sprintf("package main\n\nfunc main() {\n%s\n}", code)
	}

	mainFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(mainFile, []byte(code), 0644); err != nil {
		return err
	}

	// Try to build (or just compile check)
	// We use "go build -o /dev/null"
	cmd := exec.Command("go", "build", "-o", os.DevNull, mainFile)
	// Important: Initialize a go.mod so imports might work if they are standard lib.
	// For external libs, this is tricky.
	// We can try to use the project's go.mod if we are in the project root?
	// But `go build` inside tmpDir won't see parent go.mod unless we use workspaces or go.mod in tmpDir.
	// For now, let's create a basic go.mod

	cmd.Dir = tmpDir
	// Init mod inside tmpDir
	modInit := exec.Command("go", "mod", "init", "doctest")
	modInit.Dir = tmpDir
	modInit.Run()

	// Capture output
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s\n%s", err, string(out))
	}
	return nil
}

func validateBash(code string) error {
	// Check if bash is available
	_, err := exec.LookPath("bash")
	if err != nil {
		return nil // Skip if no bash
	}

	// Syntax check: bash -n -c "code"
	cmd := exec.Command("bash", "-n", "-c", code)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", string(out))
	}
	return nil
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}
