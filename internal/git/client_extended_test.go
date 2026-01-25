package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_Extended_ConfigGlobal(t *testing.T) {
	// ConfigGlobal and ConfigAddGlobal execute "git config --global" which modifies user's .gitconfig.
	// We should avoid modifying the actual environment.
	// We can set HOME to a temp dir to isolate global config.
	tmpHome, err := os.MkdirTemp("", "git-test-home")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tmpHome)

	t.Setenv("HOME", tmpHome)

	c := NewClient()

	// Test ConfigGlobal
	if err := c.ConfigGlobal("user.name", "Global User"); err != nil {
		t.Errorf("ConfigGlobal failed: %v", err)
	}

	// Verify
	cmd := exec.Command("git", "config", "--global", "user.name")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "Global User" {
		t.Errorf("Expected Global User, got %s", out)
	}

	// Test ConfigAddGlobal
	if err := c.ConfigAddGlobal("alias.co", "checkout"); err != nil {
		t.Errorf("ConfigAddGlobal failed: %v", err)
	}
}

func TestClient_Extended_Log(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Create some commits
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "commit 2")

	// Test Log
	lines, err := c.Log(localDir, "-n", "2", "--oneline")
	if err != nil {
		t.Errorf("Log failed: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
}

func TestClient_Extended_Diff(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "c1")

	// Get SHA
	sha1, _ := c.CurrentCommitSHA(localDir)

	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "c2")

	sha2, _ := c.CurrentCommitSHA(localDir)

	// Test Diff
	diff, err := c.Diff(localDir, sha1, sha2)
	if err != nil {
		t.Errorf("Diff failed: %v", err)
	}
	if !strings.Contains(diff, "v1") || !strings.Contains(diff, "v2") {
		t.Errorf("Diff content mismatch")
	}

	// Test DiffStat
	stat, err := c.DiffStat(localDir, sha1, sha2)
	if err != nil {
		t.Errorf("DiffStat failed: %v", err)
	}
	if !strings.Contains(stat, "f1") {
		t.Errorf("DiffStat content mismatch")
	}
}

func TestClient_Extended_Tags(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "c1")
	c.Push(localDir, "master") // Assuming master from setupTestRepo

	// Test Tag
	if err := c.Tag(localDir, "v1.0.0"); err != nil {
		t.Errorf("Tag failed: %v", err)
	}

	// Test LatestTag
	tag, err := c.LatestTag(localDir)
	if err != nil {
		t.Errorf("LatestTag failed: %v", err)
	}
	if tag != "v1.0.0" {
		t.Errorf("Expected v1.0.0, got %s", tag)
	}

	// Test PushTags
	if err := c.PushTags(localDir); err != nil {
		t.Errorf("PushTags failed: %v", err)
	}

	// Verify on remote
	cmd := exec.Command("git", "tag")
	cmd.Dir = remoteDir
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "v1.0.0") {
		t.Errorf("Tag not pushed to remote")
	}

	// Test DeleteTag
	if err := c.DeleteTag(localDir, "v1.0.0"); err != nil {
		t.Errorf("DeleteTag failed: %v", err)
	}
	tag, _ = c.LatestTag(localDir)
	if tag != "" {
		t.Errorf("Tag not deleted locally")
	}
}

func TestClient_Extended_Recover(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-test-recover")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create fake .git structure
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	// Create lock files
	locks := []string{
		filepath.Join(gitDir, "index.lock"),
		filepath.Join(gitDir, "HEAD.lock"),
		filepath.Join(gitDir, "config.lock"),
	}

	for _, l := range locks {
		if err := os.WriteFile(l, []byte{}, 0644); err != nil {
			t.Fatalf("failed to create lock file: %v", err)
		}
	}

	c := NewClient()
	if err := c.Recover(tmpDir); err != nil {
		t.Errorf("Recover failed: %v", err)
	}

	for _, l := range locks {
		if _, err := os.Stat(l); !os.IsNotExist(err) {
			t.Errorf("Lock file %s was not removed", l)
		}
	}
}

func TestClient_Extended_CurrentCommitSHA(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "c1")

	sha, err := c.CurrentCommitSHA(localDir)
	if err != nil {
		t.Errorf("CurrentCommitSHA failed: %v", err)
	}
	if len(sha) < 7 { // Full SHA is 40 chars
		t.Errorf("Invalid SHA: %s", sha)
	}
}

func TestClient_Extended_Bisect(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Create commits
	os.WriteFile(filepath.Join(localDir, "f"), []byte("v1"), 0644)
	c.Commit(localDir, "v1")
	sha1, _ := c.CurrentCommitSHA(localDir)

	os.WriteFile(filepath.Join(localDir, "f"), []byte("v2"), 0644)
	c.Commit(localDir, "v2")
	sha2, _ := c.CurrentCommitSHA(localDir)

	os.WriteFile(filepath.Join(localDir, "f"), []byte("v3"), 0644)
	c.Commit(localDir, "v3")
	sha3, _ := c.CurrentCommitSHA(localDir)

	// Start Bisect
	if err := c.BisectStart(localDir, sha3, sha1); err != nil {
		t.Errorf("BisectStart failed: %v", err)
	}

	// Mark bad
	if err := c.BisectBad(localDir, sha2); err != nil {
		t.Errorf("BisectBad failed: %v", err)
	}

	// Log
	log, err := c.BisectLog(localDir)
	if err != nil {
		t.Errorf("BisectLog failed: %v", err)
	}
	if len(log) == 0 {
		t.Error("BisectLog empty")
	}

	// Reset
	if err := c.BisectReset(localDir); err != nil {
		t.Errorf("BisectReset failed: %v", err)
	}
}

func TestClient_Extended_RemoteBranchExists(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()
	os.WriteFile(filepath.Join(localDir, "f"), []byte("v1"), 0644)
	c.Commit(localDir, "init")
	c.Push(localDir, "master")

	exists, err := c.RemoteBranchExists(localDir, "origin", "master")
	if err != nil {
		t.Errorf("RemoteBranchExists failed: %v", err)
	}
	if !exists {
		t.Error("Remote branch should exist")
	}

	exists, err = c.RemoteBranchExists(localDir, "origin", "nonexistent")
	if err != nil {
		t.Errorf("RemoteBranchExists failed: %v", err)
	}
	if exists {
		t.Error("Remote branch should not exist")
	}
}

func TestClient_Extended_Clone_Timeout(t *testing.T) {
	// This tests the context timeout logic if we could inject a long running command.
	// Since we can't easily mock exec.Command inside the client without structural changes (interface for command execution),
	// we'll rely on the happy path test in client_test.go.
	// But we can verify Clone accepts a context.
	// Actually Clone creates its own context with timeout.
	// c.Clone(ctx, ...)

	// Just a simple smoke test that Clone can be called with a cancelled context (though it creates its own child context, so parent cancel should propagate)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewClient()
	// Using a dummy url
	err := c.Clone(ctx, "http://example.com/dummy.git", "dummy-dest")
	if err == nil {
		t.Error("Expected error with cancelled context")
	}
}
