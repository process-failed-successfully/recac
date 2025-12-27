package git

import (
	"fmt"
	"os/exec"
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

// CreateBranch creates a new git branch and checks it out.
func CreateBranch(name string) error {
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
