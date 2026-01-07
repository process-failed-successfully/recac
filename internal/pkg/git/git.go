package git

import (
	"fmt"
	"os/exec"
)

// GitOps defines the interface for git operations.
type GitOps interface {
	Push(branch string) error
	CreatePR(branch, title, body string) error
	CreateBranch(name string) error
	AbortFeature(branchName string) error
}

// RealGitOps implements GitOps using actual git commands.
type RealGitOps struct{}

// Push pushes the current branch to origin.
func (r *RealGitOps) Push(branch string) error {
	cmd := exec.Command("git", "push", "origin", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w (output: %s)", branch, err, string(output))
	}
	return nil
}

// AbortFeature switches to main and deletes the feature branch.
func (r *RealGitOps) AbortFeature(branchName string) error {
	// Switch to main branch
	cmd := exec.Command("git", "checkout", "main")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to switch to main branch: %w (output: %s)", err, string(output))
	}

	// Delete the feature branch
	cmd = exec.Command("git", "branch", "-D", branchName)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w (output: %s)", branchName, err, string(output))
	}
	return nil
}

// CreatePR creates a pull request (mocked).
func (r *RealGitOps) CreatePR(branch, title, body string) error {
	fmt.Printf("Mock: Creating Pull Request for %s\n", branch)
	fmt.Printf("Title: %s\n", title)
	fmt.Printf("Body: %s\n", body)
	return nil
}

// CreateBranch creates a new git branch and checks it out.
func (r *RealGitOps) CreateBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w (output: %s)", name, err, string(output))
	}
	return nil
}

// DefaultGitOps is the default implementation.
var DefaultGitOps GitOps = &RealGitOps{}

// CreateBranch creates a new git branch and checks it out using DefaultGitOps.
func CreateBranch(name string) error {
	return DefaultGitOps.CreateBranch(name)
}

// Push pushes the current branch to origin (uses DefaultGitOps).
func Push(branch string) error {
	return DefaultGitOps.Push(branch)
}

// CreatePR creates a pull request (uses DefaultGitOps).
func CreatePR(branch, title, body string) error {
	return DefaultGitOps.CreatePR(branch, title, body)
}

// AbortFeature uses DefaultGitOps to abort a feature.
func AbortFeature(branchName string) error {
	return DefaultGitOps.AbortFeature(branchName)
}
