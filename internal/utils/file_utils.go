package utils

import (
	"bufio"
	"fmt"
	"os"
)

// ReadLines reads a whole file into memory and returns a slice of its lines.
func ReadLines(path string) ([]string, error) {
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

// WriteLines writes the lines to the given path.
func WriteLines(path string, lines []string) error {
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

// IsBinaryExt checks if the file extension corresponds to a binary file.
func IsBinaryExt(ext string) bool {
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".jpg", ".png", ".gif", ".pdf", ".zip", ".tar", ".gz", ".iso", ".class", ".jar":
		return true
	}
	return false
}

// IsBinaryContent checks the first few bytes of a file to see if it contains null bytes, indicating binary content.
func IsBinaryContent(content []byte) bool {
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
