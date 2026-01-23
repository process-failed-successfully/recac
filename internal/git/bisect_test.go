package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_BisectOperations(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// create some commits
	// commit 1 (Good)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")
	rev1, _ := c.CurrentCommitSHA(localDir)

	// commit 2 (Good)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "commit 2")

	// commit 3 (Bad)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v3"), 0644)
	c.Commit(localDir, "commit 3")
	rev3, _ := c.CurrentCommitSHA(localDir)

	// Test BisectStart
	if err := c.BisectStart(localDir, rev3, rev1); err != nil {
		t.Fatalf("BisectStart failed: %v", err)
	}

	// Verify we are in bisect mode
	cmd := exec.Command("git", "status")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if !strings.Contains(string(out), "bisect") {
		t.Errorf("Expected to be in bisect mode, got: %s", string(out))
	}

	// At this point, git should have checked out commit 2.
	// We mark it as Good.
	if err := c.BisectGood(localDir, ""); err != nil {
		t.Errorf("BisectGood failed: %v", err)
	}

	// Now bisect should be done, and it should tell us that rev3 is the first bad commit.
	// BisectLog should show the steps.
	logs, err := c.BisectLog(localDir)
	if err != nil {
		t.Fatalf("BisectLog failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("BisectLog returned empty logs")
	}

	// Test BisectReset
	if err := c.BisectReset(localDir); err != nil {
		t.Fatalf("BisectReset failed: %v", err)
	}

	// Verify we are NOT in bisect mode
	cmd = exec.Command("git", "status")
	cmd.Dir = localDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.Contains(string(out), "bisect") {
		t.Errorf("Expected to NOT be in bisect mode, got: %s", string(out))
	}
}

func TestClient_BisectBadPath(t *testing.T) {
	// Test the other path where we mark something as bad
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	c := NewClient()

	// commit 1 (Good)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")
	rev1, _ := c.CurrentCommitSHA(localDir)

	// commit 2 (Bad)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "commit 2")

	// commit 3 (Bad)
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v3"), 0644)
	c.Commit(localDir, "commit 3")
	rev3, _ := c.CurrentCommitSHA(localDir)

	// Bisect start 3(Bad) 1(Good) -> Should checkout 2
	c.BisectStart(localDir, rev3, rev1)

	// Mark 2 as Bad
	if err := c.BisectBad(localDir, ""); err != nil {
		t.Errorf("BisectBad failed: %v", err)
	}

	// Should be done, 2 is the first bad commit.
	c.BisectReset(localDir)
}
