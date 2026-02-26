package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	tea "github.com/charmbracelet/bubbletea"
)

// Reuse MockReviewAgent from review_interactive_test.go if accessible (same package)
// Assuming it is accessible since it's in package main and not in a _test.go file?
// Wait, MockReviewAgent is defined in review_interactive_test.go. Types defined in _test.go files are only visible to tests in the same package.
// Since security_fix_test.go is also package main, it should be visible when running tests.

func TestSecurityFixCmd(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "vulnerable.go")
	content := "package main\n\nfunc main() {\n  aws_key := \"AKIAABCDEFGHIJKLMNOP\" // Potential secret\n}"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Prepare mock agent response
	issues := []ReviewIssue{
		{
			File:            filePath,
			Line:            4,
			Title:           "Fix: AWS Access Key",
			Description:     "Hardcoded AWS key detected.",
			Severity:        "CRITICAL",
			Suggestion:      "Use env var",
			Replacement:     "  aws_key := os.Getenv(\"AWS_ACCESS_KEY_ID\")",
			OriginalContent: "  aws_key := \"AKIAABCDEFGHIJKLMNOP\" // Potential secret",
		},
	}
	jsonBytes, _ := json.Marshal(issues)
	mockAgent := &MockReviewAgent{ResponseJSON: string(jsonBytes)}

	// Override factories
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Override TUI runner
	originalTUIFunc := runReviewTUIFunc
	var capturedModel tea.Model
	runReviewTUIFunc = func(m tea.Model) error {
		capturedModel = m
		return nil
	}
	defer func() { runReviewTUIFunc = originalTUIFunc }()

	// Reset securityFix flag
	securityFix = false
	defer func() { securityFix = false }()

	// Execute Command
	// We need to trigger securityCmd.RunE
	// But we need to set securityFix = true.
	// Since flags are parsed by Cobra, we can set the flag via args.

	// Construct a fresh command to avoid global state issues if possible?
	// But securityCmd is global variable.
	// We can manually set the flag variable if we call RunE directly.
	// But it's better to go through Execute() logic if possible.

	// Let's use the global securityCmd but make sure to reset everything.
	// We'll set the flag manually because we are calling RunE directly?
	// No, if we call Execute, Cobra parses flags.

	// Create a dummy root command to house securityCmd for this test?
	// securityCmd is already added to rootCmd.

	// Let's create a wrapper command to avoid polluting global rootCmd state if we execute it.
	// Actually, just executing securityCmd with args works.

	// Capture output
	buf := new(bytes.Buffer)
	securityCmd.SetOut(buf)
	securityCmd.SetErr(buf)

	// Mock regex scanner? No, rely on the real one.
	// The file content contains "12345678901234567890" which might match `reGenericAPIToken`.
	// reGenericAPIToken = regexp.MustCompile(`(api|access)[_-]?key\s*[:=]\s*['"][a-zA-Z0-9_\-]{20,}['"]`)
	// "apiKey :=" matches. "12345678901234567890" is 20 chars.

	// Need to make sure scanner finds it.

	// Run
	securityCmd.SetArgs([]string{"--fix"})

	// We need to temporarily set the CWD to tempDir so the scanner finds the file easily
	// OR we pass the path to scanner. `runSecurityScan` takes `root`.
	// securityCmd scans ".".

	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// Set flag manually since we are bypassing Cobra's Execute
	securityFix = true

	if err := securityCmd.RunE(securityCmd, []string{}); err != nil {
		t.Fatalf("RunE failed: %v", err)
	}

	// Verify TUI was called
	if capturedModel == nil {
		t.Logf("Output buffer: %s", buf.String())
		t.Fatal("TUI was not started")
	}

	// Verify model content
	reviewModel, ok := capturedModel.(ReviewModel)
	if !ok {
		t.Fatal("Captured model is not of type ReviewModel")
	}

	if len(reviewModel.issues) != 1 {
		t.Errorf("Expected 1 issue, got %d", len(reviewModel.issues))
	}
	if reviewModel.issues[0].Title != "Fix: AWS Access Key" {
		t.Errorf("Expected title 'Fix: AWS Access Key', got '%s'", reviewModel.issues[0].Title)
	}
}
