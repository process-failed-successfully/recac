package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestInfoCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-info-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	// Create some files
	if err := os.WriteFile("main.go", []byte("package main\n\n// TODO: do something\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("README.md", []byte("# Test Project\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Ignored dir
	if err := os.Mkdir("node_modules", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("node_modules/test.js", []byte("console.log('ignored');\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock Git Client
	originalGitFactory := gitClientFactory
	defer func() {
		gitClientFactory = originalGitFactory
	}()

	mockGit := &MockGitClient{
		RepoExistsFunc: func(repoPath string) bool { return true },
		CurrentBranchFunc: func(repoPath string) (string, error) { return "feature/test-branch", nil },
		RunFunc: func(repoPath string, args ...string) (string, error) {
			if len(args) > 0 {
				if args[0] == "rev-list" {
					return "42\n", nil
				}
				if args[0] == "status" {
					// 1 staged (M ), 1 unstaged ( M), 1 untracked (??)
					return "M  file1.go\n M file2.go\n?? file3.go\n", nil
				}
			}
			return "", nil
		},
	}

	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// Helper to capture output
	execute := func(cmdArgs []string) (string, error) {
		rootCmd.SetArgs(cmdArgs)
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		err := rootCmd.Execute()
		return buf.String(), err
	}

	t.Run("Run info command", func(t *testing.T) {
		out, err := execute([]string{"info"})
		if err != nil {
			t.Fatalf("Failed to execute info cmd: %v", err)
		}

		// 2 files (main.go, README.md), node_modules is ignored
		// main.go has 4 lines, README.md has 1 line = 5 total lines
		// TODO count should be 1

		if !strings.Contains(out, "Repository Info:") {
			t.Errorf("Expected 'Repository Info:' in output, got: %s", out)
		}
		if !strings.Contains(out, "Git Branch:") || !strings.Contains(out, "feature/test-branch") {
			t.Errorf("Expected branch feature/test-branch in output, got: %s", out)
		}
		if !strings.Contains(out, "Commits:") || !strings.Contains(out, "42") {
			t.Errorf("Expected 42 commits in output, got: %s", out)
		}
		if !strings.Contains(out, "Staged Changes:") || !strings.Contains(out, "1") {
			t.Errorf("Expected 1 staged change in output, got: %s", out)
		}
		if !strings.Contains(out, "Unstaged Changes:") || !strings.Contains(out, "2") {
			t.Errorf("Expected 2 unstaged changes in output, got: %s", out)
		}
		if !strings.Contains(out, "Total Files:") || !strings.Contains(out, "2") {
			t.Errorf("Expected 2 total files in output, got: %s", out)
		}
		if !strings.Contains(out, "Total Lines:") || !strings.Contains(out, "5") {
			t.Errorf("Expected 5 total lines in output, got: %s", out)
		}
		if !strings.Contains(out, "TODO Count:") || !strings.Contains(out, "1") {
			t.Errorf("Expected 1 TODO in output, got: %s", out)
		}
	})

	t.Run("Not a git repo", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return false }
		out, err := execute([]string{"info"})
		if err != nil {
			t.Fatalf("Failed to execute info cmd: %v", err)
		}

		if !strings.Contains(out, "Git Branch:") || !strings.Contains(out, "N/A") {
			t.Errorf("Expected branch N/A in output, got: %s", out)
		}
		if strings.Contains(out, "Commits:") {
			t.Errorf("Expected no commits line, got: %s", out)
		}
		if strings.Contains(out, "Staged Changes:") {
			t.Errorf("Expected no staged changes line, got: %s", out)
		}
	})
}
