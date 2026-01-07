package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Signature represents a person's signature in a commit.
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

// Commit represents a single commit in a repository.
type Commit struct {
	Hash    string
	Author  Signature
	Message string
}

// GitOps defines the interface for git operations.
type GitOps interface {
	Push(branch string) error
	CreatePR(branch, title, body string) error
	CreateBranch(name string) error
	ListBranches(prefix string) ([]string, error)
	GetLastCommit(branch string) (*Commit, error)
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

// ListBranches lists all local branches with a given prefix.
func (r *RealGitOps) ListBranches(prefix string) ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname)", "refs/heads/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w (output: %s)", err, string(output))
	}

	var branches []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		branchName := scanner.Text()
		if strings.HasPrefix(branchName, "refs/heads/"+prefix) {
			branches = append(branches, branchName)
		}
	}

	return branches, nil
}

// GetLastCommit returns the last commit for a given branch.
func (r *RealGitOps) GetLastCommit(branch string) (*Commit, error) {
	// Format: HASH%nAUTHOR_NAME%nAUTHOR_EMAIL%nAUTHOR_DATE_UNIX%nMESSAGE
	format := "%H%n%an%n%ae%n%at%n%s"
	cmd := exec.Command("git", "log", "-1", fmt.Sprintf("--pretty=format:%s", format), branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get last commit for branch %s: %w (output: %s)", branch, err, string(output))
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 5 {
		return nil, fmt.Errorf("unexpected git log output format for branch %s: found %d lines, expected at least 5", branch, len(lines))
	}

	timestamp, err := strconv.ParseInt(lines[3], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("failed to parse commit timestamp: %w", err)
	}

	commit := &Commit{
		Hash: lines[0],
		Author: Signature{
			Name:  lines[1],
			Email: lines[2],
			When:  time.Unix(timestamp, 0),
		},
		Message: strings.Join(lines[4:], "\n"),
	}

	return commit, nil
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

// ListBranches lists branches using DefaultGitOps.
func ListBranches(prefix string) ([]string, error) {
	return DefaultGitOps.ListBranches(prefix)
}

// GetLastCommit gets the last commit using DefaultGitOps.
func GetLastCommit(branch string) (*Commit, error) {
	return DefaultGitOps.GetLastCommit(branch)
}
