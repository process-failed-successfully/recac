package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCodebaseContext(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go":          "package main\nfunc main() {}",
		"README.md":        "# Project",
		"ignored.txt":      "should be ignored",
		"dir/sub.go":       "package dir",
		"dir/ignored_dir/": "", // Directory
		".hidden":          "hidden file",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if strings.HasSuffix(path, "/") {
			os.MkdirAll(fullPath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(fullPath), 0755)
			os.WriteFile(fullPath, []byte(content), 0644)
		}
	}

	// Change to tmpDir so "." works as expected
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	os.Chdir(tmpDir)

	tests := []struct {
		name          string
		opts          ContextOptions
		contains      []string
		notContains   []string
		errorExpected bool
	}{
		{
			name: "Default options",
			opts: ContextOptions{
				Roots: []string{"."},
				Tree:  true,
			},
			contains: []string{
				"main.go",
				"package main",
				"README.md",
				"# Project",
				"dir/sub.go",
				"# File Tree",
			},
			notContains: []string{
				".hidden", // Hidden files skipped by default
			},
		},
		{
			name: "Ignore pattern",
			opts: ContextOptions{
				Roots:  []string{"."},
				Ignore: []string{"ignored.txt"},
				Tree:   true,
			},
			contains: []string{
				"main.go",
			},
			notContains: []string{
				"ignored.txt",
				"should be ignored",
			},
		},
		{
			name: "No content",
			opts: ContextOptions{
				Roots:     []string{"."},
				Tree:      true,
				NoContent: true,
			},
			contains: []string{
				"# File Tree",
				"main.go",
			},
			notContains: []string{
				"package main", // No content
			},
		},
		{
			name: "Max size",
			opts: ContextOptions{
				Roots:   []string{"."},
				MaxSize: 5, // Very small
			},
			contains: []string{
				// Small enough files? README.md is 9 bytes. main.go is > 20.
				// Wait, README.md is "# Project" (9 chars)
			},
			notContains: []string{
				"package main", // > 5 bytes
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := GenerateCodebaseContext(tt.opts)
			if (err != nil) != tt.errorExpected {
				t.Errorf("expected error: %v, got: %v", tt.errorExpected, err)
			}

			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("expected output to contain %q", c)
				}
			}

			for _, c := range tt.notContains {
				if strings.Contains(result, c) {
					t.Errorf("expected output NOT to contain %q", c)
				}
			}
		})
	}
}
