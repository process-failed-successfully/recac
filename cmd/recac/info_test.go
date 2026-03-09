package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"
	"recac/internal/runner"
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
		RepoExistsFunc:    func(repoPath string) bool { return true },
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

	// Mock Session Manager
	originalSessionManagerFactory := sessionManagerFactory
	defer func() {
		sessionManagerFactory = originalSessionManagerFactory
	}()

	mockSessionManager := NewMockSessionManager()
	mockSessionManager.Sessions = map[string]*runner.SessionState{
		"session1": {
			Name:           "session1",
			AgentStateFile: "session1_state.json",
		},
		"session2": {
			Name:           "session2",
			AgentStateFile: "session2_state.json",
		},
		"session3": {
			Name:           "session3",
			AgentStateFile: "",
		},
	}

	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSessionManager, nil
	}

	// Mock loadAgentState
	originalLoadAgentState := loadAgentState
	defer func() {
		loadAgentState = originalLoadAgentState
	}()

	loadAgentState = func(path string) (*agent.State, error) {
		if path == "session1_state.json" {
			return &agent.State{
				Model: "gemini-1.5-pro-latest",
				TokenUsage: agent.TokenUsage{
					TotalPromptTokens:   100,
					TotalResponseTokens: 200,
				},
			}, nil
		} else if path == "session2_state.json" {
			return &agent.State{
				Model: "gpt-4o",
				TokenUsage: agent.TokenUsage{
					TotalPromptTokens:   50,
					TotalResponseTokens: 150,
				},
			}, nil
		}
		return nil, os.ErrNotExist
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
		if !strings.Contains(out, "Total Files:") {
			t.Errorf("Expected total files in output, got: %s", out)
		}
		if !strings.Contains(out, "Total Lines:") {
			t.Errorf("Expected total lines in output, got: %s", out)
		}
		if !strings.Contains(out, "TODO Count:") || !strings.Contains(out, "1") {
			t.Errorf("Expected 1 TODO in output, got: %s", out)
		}

		if !strings.Contains(out, "Total Sessions:") || !strings.Contains(out, "3") {
			t.Errorf("Expected 3 Total Sessions in output, got: %s", out)
		}

		// Expected cost:
		// session1 (gemini-1.5-pro-latest): 100/1M * 7 + 200/1M * 21 = 0.0007 + 0.0042 = 0.0049
		// session2 (gpt-4o): 50/1M * 5 + 150/1M * 15 = 0.00025 + 0.00225 = 0.0025
		// total = 0.0074 => rounded to $0.01
		if !strings.Contains(out, "Estimated AI Cost:") || !strings.Contains(out, "$0.01") {
			t.Errorf("Expected Estimated AI Cost: $0.01 in output, got: %s", out)
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
