package git

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Client handles git interactions.
type Client struct{}

// NewClient creates a new Git client.
func NewClient() *Client {
	return &Client{}
}

// maskingWriter wraps an io.Writer and masks sensitive information.
type maskingWriter struct {
	w io.Writer
}

var (
	reGitHubPAT = regexp.MustCompile(`https://[^@:]+@github\.com`)
	reBasicAuth = regexp.MustCompile(`https://[^:/]+:[^@/]+@`)
)

func (mw *maskingWriter) Write(p []byte) (n int, err error) {
	s := string(p)
	// Mask GitHub PATs in URLs: https://<token>@github.com/
	s = reGitHubPAT.ReplaceAllString(s, "https://[REDACTED]@github.com")

	// Also mask basic auth style: https://user:pass@host
	s = reBasicAuth.ReplaceAllString(s, "https://[REDACTED]@")

	_, err = mw.w.Write([]byte(s))
	return len(p), err
}

func (c *Client) runWithMasking(ctx context.Context, dir string, args ...string) error {
	var outBuf, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Enforce no prompting
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_ASKPASS=/bin/true")
	cmd.Stdout = &maskingWriter{w: io.MultiWriter(os.Stdout, &outBuf)}
	cmd.Stderr = &maskingWriter{w: io.MultiWriter(os.Stderr, &errBuf)}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("git %s failed: %w\nOutput: %s\nStderr: %s", args[0], err, outBuf.String(), errBuf.String())
	}
	return nil
}

// Clone clones a repository into a destination directory.
func (c *Client) Clone(ctx context.Context, url, dest string) error {
	// Clone can take a while
	cloneCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return c.runWithMasking(cloneCtx, "", "clone", url, dest)
}

// CheckoutNewBranch creates and switches to a new branch.
func (c *Client) CheckoutNewBranch(dir, branchName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "checkout", "-B", branchName)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Config sets a git configuration value.
func (c *Client) Config(dir, key, value string) error {
	return c.runWithMasking(context.Background(), dir, "config", key, value)
}

// ConfigGlobal sets a global git configuration value.
func (c *Client) ConfigGlobal(key, value string) error {
	return c.runWithMasking(context.Background(), "", "config", "--global", key, value)
}

// ConfigAdd adds a value to a git configuration key.
func (c *Client) ConfigAddGlobal(key, value string) error {
	return c.runWithMasking(context.Background(), "", "config", "--global", "--add", key, value)
}

// Push pushes the branch to the remote origin.
func (c *Client) Push(dir, branchName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return c.runWithMasking(ctx, dir, "push", "-u", "origin", branchName)
}

// CreatePR creates a pull request using the GitHub CLI (gh).
// It returns the URL of the created PR.
func (c *Client) CreatePR(dir, title, body, base string) (string, error) {
	// check if gh is installed or authenticated?
	// We'll just try running it.

	// gh pr create --title "..." --body "..." --fill
	// Using --fill might be risky if we want specific title/body, but let's try to pass them.
	// If title/body are provided, --fill might be ignored or supplemental.
	// The requirement is "PR linked in ticket comment".

	args := []string{"pr", "create"}
	if title != "" {
		args = append(args, "--title", title)
	}
	if body != "" {
		args = append(args, "--body", body)
	} else {
		args = append(args, "--fill")
	}
	if base != "" {
		args = append(args, "--base", base)
	}

	cmd := exec.Command("gh", args...)
	cmd.Dir = dir

	// Capture stdout to get the URL
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh pr create failed: %w", err)
	}

	// Output is usually the URL (and potentially some other text, but usually just URL on last line).
	output := strings.TrimSpace(out.String())
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return lines[len(lines)-1], nil // URL is typically the last line
	}

	return output, nil
}

// Commit stages all changes and commits them with the given message.
func (c *Client) Commit(dir, message string) error {
	// git add .
	addCmd := exec.Command("git", "add", ".")
	addCmd.Dir = dir
	addCmd.Stdout = os.Stdout
	addCmd.Stderr = os.Stderr
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("git add failed: %w", err)
	}

	// git commit -m "message"
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	// Ensure we don't fail if there's nothing to commit, although usually we want to know.
	// But for automation, maybe we just ignore error?
	// Let's return error so we know.
	return commitCmd.Run()
}

// SetRemoteURL updates the remote URL (e.g. to include auth token).
func (c *Client) SetRemoteURL(dir, name, url string) error {
	cmd := exec.Command("git", "remote", "set-url", name, url)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Checkout switches to an existing branch.
func (c *Client) Checkout(dir, branchName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "checkout", branchName)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Pull pulls changes from the remote repository.
func (c *Client) Pull(dir, remote, branchName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return c.runWithMasking(ctx, dir, "pull", remote, branchName)
}

// Merge merges the specified branch into the current branch.
func (c *Client) Merge(dir, branchName string) error {
	cmd := exec.Command("git", "merge", branchName)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Fetch fetches changes from the remote repository.
func (c *Client) Fetch(dir, remote, branchName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return c.runWithMasking(ctx, dir, "fetch", remote, branchName)
}

// RemoteBranchExists checks if a branch exists on the remote.
func (c *Client) RemoteBranchExists(dir, remote, branch string) (bool, error) {
	// git ls-remote --heads remote branch
	cmd := exec.Command("git", "ls-remote", "--heads", remote, branch)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return false, err
	}
	output := strings.TrimSpace(out.String())
	return output != "", nil
}

// LocalBranchExists checks if a branch exists locally.
func (c *Client) LocalBranchExists(dir, branch string) (bool, error) {
	// git show-ref --verify refs/heads/branch
	cmd := exec.Command("git", "show-ref", "--verify", "refs/heads/"+branch)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// RepoExists checks if the directory is a git repository.
func (c *Client) RepoExists(dir string) bool {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return false
	}
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// CurrentBranch returns the name of the current branch.
func (c *Client) CurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// Stash stashes local changes, including untracked files.
func (c *Client) Stash(dir string) error {
	cmd := exec.Command("git", "stash", "--include-untracked")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// StashPop pops the latest stash.
func (c *Client) StashPop(dir string) error {
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Recover attempts to fix common git errors by removing lock files.
func (c *Client) Recover(dir string) error {
	locks := []string{
		".git/index.lock",
		".git/HEAD.lock",
		".git/config.lock",
		".git/refs/heads/*.lock", // Wildcards don't work with os.Remove, need manual handling if we were serious, but commonly it's index.lock
	}

	for _, lock := range locks {
		path := filepath.Join(dir, lock)
		if strings.Contains(path, "*") {
			// Skip wildcards for simple implementation for now, or use Glob
			continue
		}
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("Recover: Removing stale lock file %s\n", path)
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove lock file %s: %w", path, err)
			}
		}
	}
	return nil
}

// ResetHard resets the current branch to the specified remote/branch, wiping local changes.
func (c *Client) ResetHard(dir, remote, branch string) error {
	// git fetch remote branch
	if err := c.Fetch(dir, remote, branch); err != nil {
		return fmt.Errorf("fetch failed during reset-hard: %w", err)
	}

	// git reset --hard remote/branch
	target := fmt.Sprintf("%s/%s", remote, branch)
	return c.runWithMasking(context.Background(), dir, "reset", "--hard", target)
}

// Clean force cleans the repository of untracked files and directories.
func (c *Client) Clean(dir string) error {
	// Pre-cleanup: Handle read-only Go module files that git clean fails on
	filepath.Walk(filepath.Join(dir, "go/pkg/mod"), func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil {
			// Try to make everything writable so we can delete it
			os.Chmod(path, 0777)
		}
		return nil
	})

	// Also try to remove go/pkg/mod manually if it exists, as it's often the culprit
	os.RemoveAll(filepath.Join(dir, "go/pkg/mod"))

	cmd := exec.Command("git", "clean", "-fdx")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AbortMerge aborts an in-progress merge.
func (c *Client) AbortMerge(dir string) error {
	cmd := exec.Command("git", "merge", "--abort")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DeleteLocalBranch deletes a local branch.
func (c *Client) DeleteLocalBranch(dir, branch string) error {
	cmd := exec.Command("git", "branch", "-D", branch)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DeleteRemoteBranch deletes a remote branch.
func (c *Client) DeleteRemoteBranch(dir, remote, branch string) error {
	return c.runWithMasking(context.Background(), dir, "push", remote, "--delete", branch)
}
