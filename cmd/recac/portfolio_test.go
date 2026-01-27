package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/agent"
	"strings"
	"testing"
)

// MockPortfolioAgent for testing
type MockPortfolioAgent struct {
	SendFunc func(ctx context.Context, content string) (string, error)
}

func (m *MockPortfolioAgent) Send(ctx context.Context, content string) (string, error) {
	if m.SendFunc != nil {
		return m.SendFunc(ctx, content)
	}
	return "Mock AI Response", nil
}

func (m *MockPortfolioAgent) SendStream(ctx context.Context, content string, callback func(string)) (string, error) {
	return m.Send(ctx, content)
}

func TestPortfolioCmd(t *testing.T) {
	// 1. Save original globals
	origExec := execCommand
	origGitFactory := gitClientFactory
	origAgentFactory := agentClientFactory

	defer func() {
		execCommand = origExec
		gitClientFactory = origGitFactory
		agentClientFactory = origAgentFactory
	}()

	// 2. Mock execCommand (for git config and git log raw)
	execCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestPortfolioHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// 3. Mock gitClientFactory (for gamify stats)
	// We use the existing MockGitClient from test_helpers_test.go
	gitClientFactory = func() IGitClient {
		return &MockGitClient{
			LogFunc: func(dir string, args ...string) ([]string, error) {
				// Return gamify-parsable log
				return []string{
					"COMMIT|abc|TestUser|2023-01-01 10:00:00 +0000|feat: cool stuff",
					"100	0	main.go",
					"50	0	readme.md",
				}, nil
			},
			RepoExistsFunc: func(path string) bool {
				return true
			},
		}
	}

	// 4. Mock agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agentAgent, error) {
		return &MockPortfolioAgent{
			SendFunc: func(ctx context.Context, content string) (string, error) {
				return "### Professional Summary\nTest User is great.\n### Key Achievements\n- Did stuff.", nil
			},
		}, nil
	}

	// 5. Run Command
	tmpFile := "test_portfolio.md"
	defer os.Remove(tmpFile)

	// Execute via rootCmd to ensure proper parsing isolation from os.Args
	// We need to pass the subcommand name "portfolio"
	rootCmd.SetArgs([]string{"portfolio", "--user", "TestUser", "--out", tmpFile})

	// Capture output to avoid polluting test logs
	_, _, _ = newRootCmd()

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// 6. Verify Output
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	sContent := string(content)
	if !strings.Contains(sContent, "# Portfolio: TestUser") {
		t.Errorf("Output missing title. Got: %s", sContent)
	}
	if !strings.Contains(sContent, "Test User is great") {
		t.Errorf("Output missing AI summary. Got: %s", sContent)
	}
	// Verify gamify stats
	if !strings.Contains(sContent, "main.go") && !strings.Contains(sContent, "go") {
		t.Errorf("Output missing language stats. Got: %s", sContent)
	}
}

// TestPortfolioHelperProcess acts as a mock for external commands
func TestPortfolioHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		os.Exit(0)
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "git":
		if len(cmdArgs) > 0 && cmdArgs[0] == "config" {
			fmt.Print("TestUser")
			os.Exit(0)
		}
		if len(cmdArgs) > 0 && cmdArgs[0] == "log" {
			// Return raw commit logs
			fmt.Println("abc1234 feat: cool stuff")
			os.Exit(0)
		}
	}
	os.Exit(0)
}

// Type alias helper since we can't import agent package with same name as variable in test function easily
// Actually we can just use "recac/internal/agent"
// and refer to it as agent.Agent
// I'll fix the import.
type agentAgent = agent.Agent
