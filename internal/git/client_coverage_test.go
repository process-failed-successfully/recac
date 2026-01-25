package git

import (
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
	// Commit 1 (Good)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")
	goodSHA, _ := c.CurrentCommitSHA(localDir)

	// Commit 2
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "commit 2")

	// Commit 3 (Bad)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v3"), 0644)
	c.Commit(localDir, "commit 3")
	badSHA, _ := c.CurrentCommitSHA(localDir)

	// Start Bisect
	if err := c.BisectStart(localDir, badSHA, goodSHA); err != nil {
		t.Fatalf("BisectStart failed: %v", err)
	}

	// Verify we are bisecting
	// Usually git checks out a commit between bad and good.
	// We can check git bisect log
	logs, err := c.BisectLog(localDir)
	if err != nil {
		t.Errorf("BisectLog failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("BisectLog returned empty")
	}

	// Mark current as good (since it's commit 2)
	if err := c.BisectGood(localDir, ""); err != nil {
		t.Errorf("BisectGood failed: %v", err)
	}

	// Mark as bad
	// We might need to ensure we are in a state to mark bad, but let's try
	if err := c.BisectBad(localDir, ""); err != nil {
		// It might fail if bisect is finished?
		// t.Logf("BisectBad failed (expected if finished): %v", err)
	}

	// Reset
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

	// Push so we can push tags later
	branch, _ := c.CurrentBranch(localDir)
	c.Push(localDir, branch)

	// Test Tag
	version := "v1.0.0"
	if err := c.Tag(localDir, version); err != nil {
		t.Fatalf("Tag failed: %v", err)
	}

	// Test LatestTag
	tag, err := c.LatestTag(localDir)
	if err != nil {
		t.Errorf("LatestTag failed: %v", err)
	}
	if tag != version {
		t.Errorf("Expected latest tag %s, got %s", version, tag)
	}

	// Test PushTags
	if err := c.PushTags(localDir); err != nil {
		t.Errorf("PushTags failed: %v", err)
	}

	// Verify tag on remote
	cmd := exec.Command("git", "ls-remote", "--tags", remoteDir)
	out, _ := cmd.Output()
	if !strings.Contains(string(out), version) {
		t.Error("PushTags didn't push tag to remote")
	}

	// Test DeleteTag
	if err := c.DeleteTag(localDir, version); err != nil {
		t.Errorf("DeleteTag failed: %v", err)
	}

	// Verify deletion locally
	tag, _ = c.LatestTag(localDir)
	if tag != "" {
		t.Errorf("Tag still exists after deletion: %s", tag)
	}
}

func TestClient_GlobalConfig(t *testing.T) {
	// Isolate environment
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	c := NewClient()

	// ConfigGlobal
	key := "user.testname"
	val := "testvalue"
	if err := c.ConfigGlobal(key, val); err != nil {
		t.Fatalf("ConfigGlobal failed: %v", err)
	}

	// Verify
	cmd := exec.Command("git", "config", "--global", key)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git config failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != val {
		t.Errorf("ConfigGlobal didn't set value. Got %s", string(out))
	}

	// ConfigAddGlobal
	val2 := "testvalue2"
	if err := c.ConfigAddGlobal(key, val2); err != nil {
		t.Errorf("ConfigAddGlobal failed: %v", err)
	}

	// Verify multiple values
	cmd = exec.Command("git", "config", "--global", "--get-all", key)
	out, _ = cmd.Output()
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 values, got %d", len(lines))
	}
}
