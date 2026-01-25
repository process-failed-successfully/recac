package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_Bisect(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Create a history of commits
	// commit 1 (good)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")
	goodSHA, _ := c.CurrentCommitSHA(localDir)

	// commit 2
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "commit 2")

	// commit 3
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v3"), 0644)
	c.Commit(localDir, "commit 3")

	// commit 4 (bad)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v4"), 0644)
	c.Commit(localDir, "commit 4")
	badSHA, _ := c.CurrentCommitSHA(localDir)

	// Test BisectStart
	if err := c.BisectStart(localDir, badSHA, goodSHA); err != nil {
		t.Fatalf("BisectStart failed: %v", err)
	}

	// Verify we are in bisect mode (head should be detached or moved)
	// Usually bisect checks out a middle commit.
	// There are 2 commits between good (1) and bad (4): 2 and 3.
	// It should check out one of them.

	// Test BisectLog
	logs, err := c.BisectLog(localDir)
	if err != nil {
		t.Errorf("BisectLog failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("BisectLog returned empty")
	}

	// Test BisectGood
	// Mark current as good
	if err := c.BisectGood(localDir, ""); err != nil {
		t.Errorf("BisectGood failed: %v", err)
	}

	// Test BisectBad
	// Mark current as bad (hypothetically, though we just marked good, just testing the command runs)
	// Warning: Git logic might complain if we mark inconsistent things, but we just check if command runs without error or with expected error.
	// Let's just reset and start again for Bad
	c.BisectReset(localDir)
	c.BisectStart(localDir, badSHA, goodSHA)

	if err := c.BisectBad(localDir, ""); err != nil {
		t.Errorf("BisectBad failed: %v", err)
	}

	// Test BisectReset
	if err := c.BisectReset(localDir); err != nil {
		t.Errorf("BisectReset failed: %v", err)
	}
}

func TestClient_Tags(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Verify no tags initially
	tag, err := c.LatestTag(localDir)
	if err != nil {
		t.Errorf("LatestTag failed: %v", err)
	}
	if tag != "" {
		t.Errorf("Expected no tag, got %s", tag)
	}

	// Test Tag
	version := "v1.0.0"
	if err := c.Tag(localDir, version); err != nil {
		t.Fatalf("Tag failed: %v", err)
	}

	// Verify LatestTag
	tag, err = c.LatestTag(localDir)
	if err != nil {
		t.Errorf("LatestTag failed: %v", err)
	}
	if tag != version {
		t.Errorf("Expected tag %s, got %s", version, tag)
	}

	// Test PushTags
	if err := c.PushTags(localDir); err != nil {
		t.Errorf("PushTags failed: %v", err)
	}

	// Verify tag on remote
	// We can use ls-remote to check
	cmd := exec.Command("git", "ls-remote", "--tags", remoteDir)
	out, _ := cmd.Output()
	if !strings.Contains(string(out), version) {
		t.Error("Tag not pushed to remote")
	}

	// Test DeleteTag
	if err := c.DeleteTag(localDir, version); err != nil {
		t.Errorf("DeleteTag failed: %v", err)
	}

	// Verify tag gone locally
	tag, _ = c.LatestTag(localDir)
	if tag != "" {
		t.Error("Tag still exists locally after delete")
	}
}

func TestClient_ConfigGlobal(t *testing.T) {
	// Create a temp home directory
	tempHome, err := os.MkdirTemp("", "git-test-home")
	if err != nil {
		t.Fatalf("failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)

	// Save original HOME
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", origHome)

	c := NewClient()

	// Test ConfigGlobal
	key := "user.testname"
	val := "Test Global User"
	if err := c.ConfigGlobal(key, val); err != nil {
		t.Fatalf("ConfigGlobal failed: %v", err)
	}

	// Verify with git config --global
	cmd := exec.Command("git", "config", "--global", key)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != val {
		t.Errorf("Expected global config %s to be %s, got %s", key, val, string(out))
	}

	// Test ConfigAddGlobal
	multiKey := "user.multitest"
	val1 := "v1"
	val2 := "v2"
	c.ConfigGlobal(multiKey, val1)
	if err := c.ConfigAddGlobal(multiKey, val2); err != nil {
		t.Errorf("ConfigAddGlobal failed: %v", err)
	}

	// Verify
	cmd = exec.Command("git", "config", "--global", "--get-all", multiKey)
	out, _ = cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 values for %s, got %d", multiKey, len(lines))
	}
}

func TestClient_Diff_Coverage(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Commit 1
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "c1")
	sha1, _ := c.CurrentCommitSHA(localDir)

	// Commit 2
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "c2")
	sha2, _ := c.CurrentCommitSHA(localDir)

	// Test Diff
	diff, err := c.Diff(localDir, sha1, sha2)
	if err != nil {
		t.Errorf("Diff failed: %v", err)
	}
	if !strings.Contains(diff, "diff --git") {
		t.Error("Diff output doesn't look like a diff")
	}

	// Test DiffStat
	stat, err := c.DiffStat(localDir, sha1, sha2)
	if err != nil {
		t.Errorf("DiffStat failed: %v", err)
	}
	if !strings.Contains(stat, "f1") {
		t.Error("DiffStat missing file name")
	}
}

func TestClient_Log_Coverage(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Test Log with args
	logs, err := c.Log(localDir, "-n", "1", "--oneline")
	if err != nil {
		t.Errorf("Log failed: %v", err)
	}
	if len(logs) != 1 {
		t.Errorf("Expected 1 log line, got %d", len(logs))
	}
}

func TestClient_Clone_Timeout(t *testing.T) {
	// This is hard to test deterministically without mocking exec.Command to hang.
	// But we can call Clone with an invalid URL and ensure it returns an error (handled by runWithMasking).

	c := NewClient()
	tmpDir, _ := os.MkdirTemp("", "git-test-clone-fail")
	defer os.RemoveAll(tmpDir)

	err := c.Clone(context.Background(), "http://invalid.example.com/repo.git", tmpDir)
	if err == nil {
		t.Error("Clone with invalid URL should fail")
	}
}
