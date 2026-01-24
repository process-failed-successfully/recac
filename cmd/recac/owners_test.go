package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchCodeOwnerPattern(t *testing.T) {
	tests := []struct {
		pattern string
		file    string
		match   bool
	}{
		{"*", "foo.go", true},
		{"*.js", "foo.js", true},
		{"*.js", "foo.go", false},
		{"docs/", "docs/foo.md", true},
		{"internal/", "internal/foo.go", true},
		{"internal/", "internal/sub/foo.go", true},
		{"/internal/", "internal/foo.go", true}, // Anchored match
	}

	for _, tt := range tests {
		m, err := matchCodeOwnerPattern(tt.pattern, tt.file)
		require.NoError(t, err)
		if m != tt.match {
			t.Errorf("matchCodeOwnerPattern(%q, %q) = %v; want %v", tt.pattern, tt.file, m, tt.match)
		}
	}
}

func TestResolveCodeOwners(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "owners-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	content := `
# Comment
* @global-owner
*.js @js-owner
/internal/ @backend-owner
`
	err = os.WriteFile(filepath.Join(tmpDir, "CODEOWNERS"), []byte(content), 0644)
	require.NoError(t, err)

	tests := []struct {
		file   string
		owners []string
	}{
		{"foo.txt", []string{"@global-owner"}},
		{"foo.js", []string{"@js-owner"}},
		{"internal/main.go", []string{"@backend-owner"}},
	}

	for _, tt := range tests {
		owners, _, err := resolveCodeOwners(tmpDir, tt.file)
		require.NoError(t, err)
		require.Equal(t, tt.owners, owners, "File: %s", tt.file)
	}
}

func TestRunOwners_GitFallback(t *testing.T) {
	// Mock git client
	oldFactory := gitClientFactory
	defer func() { gitClientFactory = oldFactory }()

	mockGit := &MockGitClient{
		RepoExistsFunc: func(dir string) bool { return true },
		LogFunc: func(dir string, args ...string) ([]string, error) {
			return []string{
				"Alice <alice@example.com>",
				"Bob <bob@example.com>",
				"Alice <alice@example.com>",
			}, nil
		},
	}
	gitClientFactory = func() IGitClient { return mockGit }

	// Mock execCommand for findRepoRoot
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Create temp dir
	cwd, _ := os.Getwd()
	tmpDir, _ := os.MkdirTemp("", "owners-cmd-test-")
	defer os.RemoveAll(tmpDir)

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			// echo tmpDir as root
			return oldExec("echo", tmpDir)
		}
		return oldExec(name, args...)
	}

	// Switch to tmpDir
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	output, err := executeCommand(rootCmd, "owners", "main.go")
	require.NoError(t, err)
	require.Contains(t, output, "No CODEOWNERS rule found")
	require.Contains(t, output, "Top contributors for main.go")
	require.Contains(t, output, "Alice <alice@example.com> (2 commits")
}

func TestGenerateOwners(t *testing.T) {
	// Mock git client
	oldFactory := gitClientFactory
	defer func() { gitClientFactory = oldFactory }()

	mockGit := &MockGitClient{
		RepoExistsFunc: func(dir string) bool { return true },
		LogFunc: func(dir string, args ...string) ([]string, error) {
			if len(args) > 0 {
				path := args[len(args)-1]
				if strings.Contains(path, "backend") {
					return []string{"backend@example.com", "backend@example.com"}, nil
				}
				if strings.Contains(path, "frontend") {
					return []string{"frontend@example.com"}, nil
				}
			}
			return []string{}, nil
		},
	}
	gitClientFactory = func() IGitClient { return mockGit }

	// Mock execCommand for findRepoRoot
	oldExec := execCommand
	defer func() { execCommand = oldExec }()

	// Setup fake fs
	tmpDir, _ := os.MkdirTemp("", "owners-gen-test-")
	defer os.RemoveAll(tmpDir)

	execCommand = func(name string, args ...string) *exec.Cmd {
		if name == "git" && len(args) > 0 && args[0] == "rev-parse" {
			return oldExec("echo", tmpDir)
		}
		return oldExec(name, args...)
	}

	os.Mkdir(filepath.Join(tmpDir, "backend"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "frontend"), 0755)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	output, err := executeCommand(rootCmd, "owners", "--generate")
	require.NoError(t, err)

	require.Contains(t, output, "backend/                       backend@example.com")
	require.Contains(t, output, "frontend/                      frontend@example.com")
}
