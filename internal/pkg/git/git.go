package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// GitOps defines the interface for git operations.
type GitOps interface {
	Push(branch string) error
	CreatePR(branch, title, body string) error
}

// RealGitOps implements GitOps using actual git commands.
type RealGitOps struct{}

// Push pushes the current branch to origin.
func (r *RealGitOps) Push(branch string) error {
	fmt.Printf("Mock: git push origin %s\n", branch)
	// In a real implementation:
	// cmd := exec.Command("git", "push", "origin", branch)
	// return cmd.Run()
	return nil
}

// CreatePR creates a pull request (mocked).
func (r *RealGitOps) CreatePR(branch, title, body string) error {
	fmt.Printf("Mock: Creating Pull Request for %s\n", branch)
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Body: %s\n", body)
	return nil
}

// DefaultGitOps is the default implementation.
var DefaultGitOps GitOps = &RealGitOps{}

// ValidateBranchName checks if a branch name is valid and safe.
// It follows git check-ref-format rules and adds stricter safety checks.
func ValidateBranchName(name string) error {
	if name == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	// 1. Must not start with '-' to prevent flag injection
	if strings.HasPrefix(name, "-") {
		return fmt.Errorf("branch name cannot start with '-'")
	}

	// 2. Must not contain '..'
	if strings.Contains(name, "..") {
		return fmt.Errorf("branch name cannot contain '..'")
	}

	// 3. Must not end with '/' or '.'
	if strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("branch name cannot end with '/' or '.'")
	}

	// 4. Must not contain control characters, spaces, or invalid git chars
	// Allowed: alphanumeric, ., -, _, /
	// Disallowed: space, ~, ^, :, ?, *, [, \, @{
	validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9._/-]+$`)
	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("branch name contains invalid characters (allowed: a-z, A-Z, 0-9, ., -, _, /)")
	}

	// 5. Must not contain double slashes '//' (git normalizes this but better to be strict)
	if strings.Contains(name, "//") {
		return fmt.Errorf("branch name cannot contain '//'")
	}

	return nil
}

// CreateBranch creates a new git branch and checks it out.
func CreateBranch(name string) error {
	// 0. Validate input
	if err := ValidateBranchName(name); err != nil {
		return fmt.Errorf("invalid branch name %q: %w", name, err)
	}

	// 1. Create branch
	cmd := exec.Command("git", "checkout", "-b", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w (output: %s)", name, err, string(output))
	}
	return nil
}

// Push pushes the current branch to origin (uses DefaultGitOps).
func Push(branch string) error {
	return DefaultGitOps.Push(branch)
}

// CreatePR creates a pull request (uses DefaultGitOps).
func CreatePR(branch, title, body string) error {
	return DefaultGitOps.CreatePR(branch, title, body)
}
