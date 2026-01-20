package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
)

func ComputeSourceHash() (string, error) {
	hasher := sha256.New()
	dirs := []string{"cmd", "internal", "pkg"}
	files := []string{"go.mod", "go.sum", "Dockerfile"}

	// Gather all files
	var allFiles []string

	// Add root files
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			allFiles = append(allFiles, f)
		}
	}

	// Add dirs recursively
	for _, d := range dirs {
		if _, err := os.Stat(d); os.IsNotExist(err) {
			continue
		}
		err := filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				allFiles = append(allFiles, path)
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}

	// Sort to ensure determinism
	sort.Strings(allFiles)

	for _, file := range allFiles {
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}

		// Hash the file path first to detect renames/moves
		hasher.Write([]byte(file))

		if _, err := io.Copy(hasher, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func GetEnvOrDefault(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
