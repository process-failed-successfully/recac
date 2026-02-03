package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_SyncBranch(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()
	ctx := context.Background()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "main.txt"), []byte("main"), 0644)
	c.Commit(localDir, "init")
	// Push main so we have a base
	c.Push(localDir, "master") // assuming master/main

	// Case 1: New Branch
	branchName := "feature-new"
	if err := c.SyncBranch(ctx, localDir, branchName, ""); err != nil {
		t.Errorf("SyncBranch failed for new branch: %v", err)
	}

	// Verify local branch
	current, _ := c.CurrentBranch(localDir)
	if current != branchName {
		t.Errorf("Expected branch %s, got %s", branchName, current)
	}

	// Verify remote existence
	exists, _ := c.RemoteBranchExists(localDir, "origin", branchName)
	if !exists {
		t.Error("SyncBranch did not push the new branch")
	}

	// Case 2: Existing Remote Branch
	// Switch back to main locally
	c.Checkout(localDir, "master")

	// Create another branch on remote via another clone (simulating teammate)
	otherDir, _ := os.MkdirTemp("", "git-test-other")
	defer os.RemoveAll(otherDir)
	c.Clone(ctx, remoteDir, otherDir)
	c.ConfigureIdentity(otherDir, "Other", "other@example.com")

	remoteBranch := "feature-existing"
	c.CheckoutNewBranch(otherDir, remoteBranch)
	os.WriteFile(filepath.Join(otherDir, "feat.txt"), []byte("feat"), 0644)
	c.Commit(otherDir, "feature commit")
	c.Push(otherDir, remoteBranch)

	// Sync local
	if err := c.SyncBranch(ctx, localDir, remoteBranch, ""); err != nil {
		t.Errorf("SyncBranch failed for existing remote branch: %v", err)
	}

	current, _ = c.CurrentBranch(localDir)
	if current != remoteBranch {
		t.Errorf("Expected branch %s, got %s", remoteBranch, current)
	}

	// Verify content
	content, _ := os.ReadFile(filepath.Join(localDir, "feat.txt"))
	if string(content) != "feat" {
		t.Error("SyncBranch did not pull content for existing branch")
	}

	// Case 3: Update existing branch
	// Push more changes to remoteBranch from otherDir
	os.WriteFile(filepath.Join(otherDir, "feat.txt"), []byte("feat-v2"), 0644)
	c.Commit(otherDir, "v2")
	c.Push(otherDir, remoteBranch)

	// Switch local back to master to simulate switching task
	c.Checkout(localDir, "master")

	// Sync again
	if err := c.SyncBranch(ctx, localDir, remoteBranch, ""); err != nil {
		t.Errorf("SyncBranch failed for update: %v", err)
	}

	content, _ = os.ReadFile(filepath.Join(localDir, "feat.txt"))
	if string(content) != "feat-v2" {
		t.Errorf("SyncBranch did not pull latest changes. Got %s", string(content))
	}
}

func TestClient_ConfigureIdentity(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	name := "Agent Smith"
	email := "smith@matrix.com"

	if err := c.ConfigureIdentity(localDir, name, email); err != nil {
		t.Fatalf("ConfigureIdentity failed: %v", err)
	}

	cmd := exec.Command("git", "config", "user.name")
	cmd.Dir = localDir
	out, _ := cmd.Output()
	if strings.TrimSpace(string(out)) != name {
		t.Errorf("user.name mismatch. Got %s", string(out))
	}

	cmd = exec.Command("git", "config", "user.email")
	cmd.Dir = localDir
	out, _ = cmd.Output()
	if strings.TrimSpace(string(out)) != email {
		t.Errorf("user.email mismatch. Got %s", string(out))
	}
}
