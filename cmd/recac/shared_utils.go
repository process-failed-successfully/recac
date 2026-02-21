package main

import (
	"fmt"
	"os"
	"os/exec"
	"recac/internal/utils"
	"regexp"
	"strings"
)

// execCommand is a package-level variable to allow mocking in tests.
var execCommand = exec.Command

// writeFileFunc is a package-level variable to allow mocking in tests.
var writeFileFunc = os.WriteFile

// mkdirAllFunc is a package-level variable to allow mocking in tests.
var mkdirAllFunc = os.MkdirAll

// readFileFunc is a package-level variable to allow mocking in tests.
var readFileFunc = os.ReadFile

// DefaultIgnoreMap returns a map of common directories and files to ignore during scans.
func DefaultIgnoreMap() map[string]bool {
	return utils.DefaultIgnoreMap()
}

// extractFileContexts scans the output for file paths and returns their content formatted for the prompt.
func extractFileContexts(output string) (string, error) {
	// Regex to find file paths like "main.go:23" or "pkg/foo/bar.js:10:5"
	// We look for alphanumeric+dots+slashes, followed by a colon and a number
	re := regexp.MustCompile(`(?m)([\w\-\./]+\.\w+):(\d+)`)

	matches := re.FindAllStringSubmatch(output, -1)
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
		content, err := readFileFunc(path)
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
