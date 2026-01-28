package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// execCommand is a package-level variable to allow mocking in tests.
var execCommand = exec.Command

// writeFileFunc is a package-level variable to allow mocking in tests.
var writeFileFunc = os.WriteFile

// mkdirAllFunc is a package-level variable to allow mocking in tests.
var mkdirAllFunc = os.MkdirAll

var (
	cachedIgnoreMap map[string]bool
	ignoreMapOnce   sync.Once
	// fileContextRegex matches file paths like "main.go:23"
	fileContextRegex = regexp.MustCompile(`(?m)([\w\-\./]+\.\w+):(\d+)`)
)

// DefaultIgnoreMap returns a map of common directories and files to ignore during scans.
func DefaultIgnoreMap() map[string]bool {
	ignoreMapOnce.Do(func() {
		cachedIgnoreMap = map[string]bool{
			".git":         true,
			"node_modules": true,
			"vendor":       true,
			"dist":         true,
			"build":        true,
			".recac":       true,
			".idea":        true,
			".vscode":      true,
			"bin":          true,
			"obj":          true,
			"__pycache__":  true,
			"TODO.md":      true,
		}
	})
	return cachedIgnoreMap
}

// readLines reads a whole file into memory and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// writeLines writes the lines to the given path.
func writeLines(path string, lines []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range lines {
		if _, err := fmt.Fprintln(w, line); err != nil {
			return err
		}
	}
	return w.Flush()
}

// isBinaryExt checks if the file extension corresponds to a binary file.
func isBinaryExt(ext string) bool {
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".jpg", ".png", ".gif", ".pdf", ".zip", ".tar", ".gz", ".iso", ".class", ".jar":
		return true
	}
	return false
}

// isBinaryContent checks the first few bytes of a file to see if it contains null bytes, indicating binary content.
func isBinaryContent(content []byte) bool {
	limit := 512
	if len(content) < limit {
		limit = len(content)
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// extractFileContexts scans the output for file paths and returns their content formatted for the prompt.
func extractFileContexts(output string) (string, error) {
	// We look for alphanumeric+dots+slashes, followed by a colon and a number
	matches := fileContextRegex.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return "No specific files identified in error output.", nil
	}

	uniqueFiles := make(map[string]bool)
	var sb strings.Builder

	for _, match := range matches {
		path := match[1]
		if uniqueFiles[path] {
			continue
		}
		uniqueFiles[path] = true

		// Validate file exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// Try relative to CWD if not found
			// (Output might be relative or absolute)
			continue
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			sb.WriteString(fmt.Sprintf("Could not read file %s: %v\n", path, err))
			continue
		}

		// Limit content size to avoid context overflow (e.g. 10KB per file)
		const maxFileSize = 10 * 1024
		fileStr := string(content)
		if len(fileStr) > maxFileSize {
			fileStr = fileStr[:maxFileSize] + "\n... (truncated)"
		}

		sb.WriteString(fmt.Sprintf("File: %s\n```\n%s\n```\n\n", path, fileStr))
	}

	if sb.Len() == 0 {
		return "Files referenced in output could not be read.", nil
	}

	return sb.String(), nil
}

// sanitizeMermaidID replaces characters invalid in Mermaid node IDs with underscores.
func sanitizeMermaidID(id string) string {
	replacer := strings.NewReplacer(
		"-", "_",
		" ", "_",
		".", "_",
		"(", "_",
		")", "_",
		"[", "_",
		"]", "_",
		"/", "_",
		"\\", "_",
		"*", "_",
		":", "_",
	)
	return replacer.Replace(id)
}
