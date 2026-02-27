package main

import (
	"recac/internal/git"
	"testing"
)

func TestFeatureStartCmd(t *testing.T) {
	// Backup and restore git.NewClient
	originalGitClient := git.NewClient
	defer func() { git.NewClient = originalGitClient }()

	var capturedBranch string

	// Setup shared mock from test_helpers_test.go
	mock := &MockGitClient{
		CheckoutNewBranchFunc: func(dir, branch string) error {
			capturedBranch = branch
			return nil
		},
	}

	// Inject mock
	git.NewClient = func() git.IClient {
		return mock
	}

	// Execute command
	// We call Run directly to test logic
	featureName := "test-feature"
	args := []string{featureName}

	// We need to capture stdout to prevent cluttering test output
	// But featureStartCmd writes to stdout using fmt.Printf.
	// We can't easily capture it unless we redirect os.Stdout or use a different output mechanism.
	// For this test, we care about the logic (git call), so we can ignore stdout or let it print.

	featureStartCmd.Run(featureStartCmd, args)

	expectedBranch := "feature/" + featureName
	if capturedBranch != expectedBranch {
		t.Errorf("Expected branch '%s', got '%s'", expectedBranch, capturedBranch)
	}
}
