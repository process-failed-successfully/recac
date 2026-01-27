package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) (string, string) {
	t.Helper()

	// Create a bare repo to act as remote
	remoteDir, err := os.MkdirTemp("", "git-test-remote")
	if err != nil {
		t.Fatalf("failed to create temp remote dir: %v", err)
	}

	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Create a local repo
	localDir, err := os.MkdirTemp("", "git-test-local")
	if err != nil {
		t.Fatalf("failed to create temp local dir: %v", err)
	}

	cmd = exec.Command("git", "init")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init local repo: %v", err)
	}

	// Configure user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = localDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = localDir
	cmd.Run()

	// Add remote
	cmd = exec.Command("git", "remote", "add", "origin", remoteDir)
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	return localDir, remoteDir
}

func TestClient_BasicOperations(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Test RepoExists
	if !c.RepoExists(localDir) {
		t.Error("RepoExists returned false for valid repo")
	}

	// Test Commit
	testFile := filepath.Join(localDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	if err := c.Commit(localDir, "Initial commit"); err != nil {
		t.Errorf("Commit failed: %v", err)
	}

	// Test CurrentBranch
	branch, err := c.CurrentBranch(localDir)
	if err != nil {
		t.Errorf("CurrentBranch failed: %v", err)
	}
	if branch == "" {
		t.Error("CurrentBranch returned empty string")
	}

	// Test Push
	// Note: 'master' or 'main' depends on git version/config. git init usually uses 'master' by default in older, 'main' in newer.
	// Let's force branch name to 'main'
	exec.Command("git", "-C", localDir, "branch", "-m", "main").Run()

	if err := c.Push(localDir, "main"); err != nil {
		t.Errorf("Push failed: %v", err)
	}

	// Test RemoteBranchExists
	exists, err := c.RemoteBranchExists(localDir, "origin", "main")
	if err != nil {
		t.Errorf("RemoteBranchExists failed: %v", err)
	}
	if !exists {
		t.Error("RemoteBranchExists returned false after push")
	}
}

func TestClient_Branching(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Initial commit needed for branching
	testFile := filepath.Join(localDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	exec.Command("git", "-C", localDir, "add", ".").Run()
	exec.Command("git", "-C", localDir, "commit", "-m", "init").Run()

	// Test CheckoutNewBranch
	if err := c.CheckoutNewBranch(localDir, "feature-1"); err != nil {
		t.Errorf("CheckoutNewBranch failed: %v", err)
	}

	branch, _ := c.CurrentBranch(localDir)
	if branch != "feature-1" {
		t.Errorf("Expected branch feature-1, got %s", branch)
	}

	// Test LocalBranchExists
	exists, err := c.LocalBranchExists(localDir, "feature-1")
	if err != nil {
		t.Errorf("LocalBranchExists failed: %v", err)
	}
	if !exists {
		t.Error("LocalBranchExists returned false for current branch")
	}

	// Test Checkout (switch back)
	// First determine default branch name
	cmd := exec.Command("git", "-C", localDir, "branch", "--list", "master")
	out, _ := cmd.Output()
	mainBranch := "master"
	if len(out) == 0 {
		mainBranch = "main"
	}

	if err := c.Checkout(localDir, mainBranch); err != nil {
		t.Errorf("Checkout failed: %v", err)
	}

	branch, _ = c.CurrentBranch(localDir)
	if branch != mainBranch {
		t.Errorf("Expected branch %s, got %s", mainBranch, branch)
	}
}

func TestClient_StashAndClean(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Modify file
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)

	// Test Stash
	if err := c.Stash(localDir); err != nil {
		t.Errorf("Stash failed: %v", err)
	}

	// Content should be v1
	content, _ := os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v1" {
		t.Errorf("Stash didn't revert changes. Got %s", string(content))
	}

	// Test StashPop
	if err := c.StashPop(localDir); err != nil {
		t.Errorf("StashPop failed: %v", err)
	}

	content, _ = os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v2" {
		t.Errorf("StashPop didn't restore changes. Got %s", string(content))
	}

	// Test Clean
	os.WriteFile(filepath.Join(localDir, "untracked.txt"), []byte("foo"), 0644)
	if err := c.Clean(localDir); err != nil {
		t.Errorf("Clean failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(localDir, "untracked.txt")); !os.IsNotExist(err) {
		t.Error("Clean didn't remove untracked file")
	}
}

func TestClient_Clone(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	// Add content to remote via local
	c := NewClient()
	os.WriteFile(filepath.Join(localDir, "readme.md"), []byte("# test"), 0644)
	c.Commit(localDir, "init")

	// Determine branch
	cmd := exec.Command("git", "-C", localDir, "branch", "--show-current")
	out, _ := cmd.Output()
	branch := strings.TrimSpace(string(out))

	c.Push(localDir, branch)

	// Test Clone
	cloneDir, err := os.MkdirTemp("", "git-test-clone")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(cloneDir)

	// Clone expects the destination dir to be either empty or non-existent
	// Since MkdirTemp creates it, it's empty. But git clone often prefers creating the directory itself if it's the last arg.
	// However, git clone <url> <dest> works if dest is empty directory.

	if err := c.Clone(context.Background(), remoteDir, cloneDir); err != nil {
		t.Errorf("Clone failed: %v", err)
	}

	if !c.RepoExists(cloneDir) {
		t.Error("Clone didn't create a valid repo")
	}
}

func TestClient_RemoteOperations(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Setup remote content
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Determine branch
	cmd := exec.Command("git", "-C", localDir, "branch", "--show-current")
	out, _ := cmd.Output()
	branch := strings.TrimSpace(string(out))

	c.Push(localDir, branch)

	// Create another client/clone to simulate another user pushing changes
	otherDir, _ := os.MkdirTemp("", "git-test-other")
	defer os.RemoveAll(otherDir)
	c.Clone(context.Background(), remoteDir, otherDir)

	// Configure user for otherDir (needed in CI)
	exec.Command("git", "-C", otherDir, "config", "user.email", "test2@example.com").Run()
	exec.Command("git", "-C", otherDir, "config", "user.name", "Test User 2").Run()

	// Other user makes changes
	os.WriteFile(filepath.Join(otherDir, "f1"), []byte("v2"), 0644)
	c.Commit(otherDir, "update v2")
	c.Push(otherDir, branch)

	// Test Fetch
	if err := c.Fetch(localDir, "origin", branch); err != nil {
		t.Errorf("Fetch failed: %v", err)
	}

	// Test Pull
	if err := c.Pull(localDir, "origin", branch); err != nil {
		t.Errorf("Pull failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v2" {
		t.Errorf("Pull didn't update content. Got %s", string(content))
	}
}

func TestClient_ResetHard(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Init
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Determine branch
	cmd := exec.Command("git", "-C", localDir, "branch", "--show-current")
	out, _ := cmd.Output()
	branch := strings.TrimSpace(string(out))

	c.Push(localDir, branch)

	// Make local changes
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("modified"), 0644)

	// Test ResetHard
	if err := c.ResetHard(localDir, "origin", branch); err != nil {
		t.Errorf("ResetHard failed: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(localDir, "f1"))
	if string(content) != "v1" {
		t.Errorf("ResetHard didn't revert changes. Got %s", string(content))
	}
}

func TestClient_SetRemoteURL(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	newURL := "https://example.com/repo.git"
	if err := c.SetRemoteURL(localDir, "origin", newURL); err != nil {
		t.Errorf("SetRemoteURL failed: %v", err)
	}

	cmd := exec.Command("git", "-C", localDir, "remote", "get-url", "origin")
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != newURL {
		t.Errorf("Remote URL not updated. Got %s", string(out))
	}
}

func TestClient_CreatePR_Skip(t *testing.T) {
	// Creating a PR requires 'gh' CLI and real auth, which we can't easily test here.
	// We'll just verify the function exists and doesn't panic on nil client,
	// but we'll likely skip or mock if we were serious.
	// Here we just skip to acknowledge we aren't covering it fully in integration test.
	t.Skip("Skipping CreatePR test as it requires gh CLI and auth")
}

func TestClient_Merge(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Init
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Create feature branch
	c.CheckoutNewBranch(localDir, "feature")
	os.WriteFile(filepath.Join(localDir, "f2"), []byte("v2"), 0644)
	c.Commit(localDir, "feature commit")

	// Switch back
	c.Checkout(localDir, "master")

	// Merge
	if err := c.Merge(localDir, "feature"); err != nil {
		t.Errorf("Merge failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filepath.Join(localDir, "f2")); os.IsNotExist(err) {
		t.Error("Merge didn't bring in changes")
	}
}

func TestClient_ErrorHandling(t *testing.T) {
	c := NewClient()
	tmpDir, _ := os.MkdirTemp("", "git-test-errors")
	defer os.RemoveAll(tmpDir)

	// RepoExists on empty dir
	if c.RepoExists(tmpDir) {
		t.Error("RepoExists returned true for empty dir")
	}

	// RepoExists on non-existent dir
	if c.RepoExists(filepath.Join(tmpDir, "nonexistent")) {
		t.Error("RepoExists returned true for nonexistent dir")
	}

	// CurrentBranch on non-repo
	if _, err := c.CurrentBranch(tmpDir); err == nil {
		t.Error("CurrentBranch should fail on non-repo")
	}

	// Commit on non-repo
	if err := c.Commit(tmpDir, "msg"); err == nil {
		t.Error("Commit should fail on non-repo")
	}
}

func TestClient_Config(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Test Config
	if err := c.Config(localDir, "user.name", "test-user"); err != nil {
		t.Fatalf("Config failed: %v", err)
	}
	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config user.name failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "test-user" {
		t.Errorf("expected user.name to be 'test-user', got '%s'", string(out))
	}
}

func TestClient_AbortMerge(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("initial"), 0644)
	c.Commit(localDir, "initial commit")

	// Create a branch
	c.CheckoutNewBranch(localDir, "feature")
	os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("feature"), 0644)
	c.Commit(localDir, "feature commit")

	// Go back to main and create a conflicting change
	mainBranch := "master" // default in test setup
	c.Checkout(localDir, mainBranch)
	os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("main"), 0644)
	c.Commit(localDir, "main commit")

	// This merge will fail
	err := c.Merge(localDir, "feature")
	if err == nil {
		t.Fatal("expected merge to fail, but it did not")
	}

	// Abort the merge
	if err := c.AbortMerge(localDir); err != nil {
		t.Fatalf("AbortMerge failed: %v", err)
	}

	// Check that the merge is no longer in progress
	cmd := exec.Command("git", "status")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.Contains(string(out), "merging") {
		t.Error("git status still shows merge in progress after abort")
	}
}

func TestClient_DeleteLocalBranch(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("initial"), 0644)
	c.Commit(localDir, "initial commit")

	// Create a branch
	c.CheckoutNewBranch(localDir, "to-delete")

	// Go back to main
	mainBranch := "master" // default in test setup
	c.Checkout(localDir, mainBranch)

	// Delete the branch
	if err := c.DeleteLocalBranch(localDir, "to-delete"); err != nil {
		t.Fatalf("DeleteLocalBranch failed: %v", err)
	}

	// Verify the branch is gone
	exists, err := c.LocalBranchExists(localDir, "to-delete")
	if err != nil {
		t.Fatalf("LocalBranchExists failed: %v", err)
	}
	if exists {
		t.Error("branch 'to-delete' still exists after deletion")
	}
}

func TestClient_DeleteRemoteBranch(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Initial commit on default branch and push
	os.WriteFile(filepath.Join(localDir, "file.txt"), []byte("initial"), 0644)
	c.Commit(localDir, "initial commit")
	mainBranch := "master" // default in test setup
	c.Push(localDir, mainBranch)

	// Create and push a new branch to be deleted
	deleteBranch := "to-delete"
	c.CheckoutNewBranch(localDir, deleteBranch)
	os.WriteFile(filepath.Join(localDir, "delete.txt"), []byte("delete me"), 0644)
	c.Commit(localDir, "commit to delete")
	c.Push(localDir, deleteBranch)

	// Go back to main branch
	c.Checkout(localDir, mainBranch)

	// Delete the remote branch
	if err := c.DeleteRemoteBranch(localDir, "origin", deleteBranch); err != nil {
		t.Fatalf("DeleteRemoteBranch failed: %v", err)
	}

	// Verify the branch is gone from the remote
	exists, err := c.RemoteBranchExists(localDir, "origin", deleteBranch)
	if err != nil {
		t.Fatalf("RemoteBranchExists failed: %v", err)
	}
	if exists {
		t.Error("remote branch still exists after deletion")
	}
}

func TestClient_DiffStaged(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Modify file
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)

	// Check diff staged before adding
	diff, err := c.DiffStaged(localDir)
	if err != nil {
		t.Fatalf("DiffStaged failed: %v", err)
	}
	if diff != "" {
		t.Errorf("Expected empty staged diff, got %s", diff)
	}

	// Stage changes
	exec.Command("git", "-C", localDir, "add", ".").Run()

	// Check diff staged after adding
	diff, err = c.DiffStaged(localDir)
	if err != nil {
		t.Fatalf("DiffStaged failed: %v", err)
	}
	if !strings.Contains(diff, "v2") {
		t.Errorf("Expected diff to contain 'v2', got %s", diff)
	}
}

func TestClient_HasUnpushedCommits(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Initial commit and push
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Determine branch
	cmd := exec.Command("git", "-C", localDir, "branch", "--show-current")
	out, _ := cmd.Output()
	branch := strings.TrimSpace(string(out))

	c.Push(localDir, branch)

	// Check unpushed (should be false)
	unpushed, err := c.HasUnpushedCommits(localDir)
	if err != nil {
		t.Fatalf("HasUnpushedCommits failed: %v", err)
	}
	if unpushed {
		t.Error("HasUnpushedCommits returned true for synced repo")
	}

	// Make changes and commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "update")

	// Check unpushed (should be true)
	unpushed, err = c.HasUnpushedCommits(localDir)
	if err != nil {
		t.Fatalf("HasUnpushedCommits failed: %v", err)
	}
	if !unpushed {
		t.Error("HasUnpushedCommits returned false for ahead repo")
	}

	// Push
	c.Push(localDir, branch)
	unpushed, _ = c.HasUnpushedCommits(localDir)
	if unpushed {
		t.Error("HasUnpushedCommits returned true after push")
	}

	// New branch (no upstream)
	c.CheckoutNewBranch(localDir, "feature-new")
	unpushed, err = c.HasUnpushedCommits(localDir)
	// Should return true (needs push to set upstream)
	if !unpushed {
		t.Error("HasUnpushedCommits returned false for new branch (no upstream)")
	}
}
