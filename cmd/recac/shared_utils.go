package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
)

// execCommand is a package-level variable to allow mocking in tests.
var execCommand = exec.Command

// lookPath is a package-level variable to allow mocking in tests.
var lookPath = exec.LookPath

// DefaultIgnoreMap returns a map of common directories and files to ignore during scans.
func DefaultIgnoreMap() map[string]bool {
	return map[string]bool{
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
