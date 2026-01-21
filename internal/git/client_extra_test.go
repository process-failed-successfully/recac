package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_Log(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")

	// Second commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "commit 2")

	// Test Log
	logs, err := c.Log(localDir, "-n", "2", "--oneline")
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	if len(logs) != 2 {
		t.Errorf("Expected 2 log lines, got %d", len(logs))
	}
	if !strings.Contains(logs[0], "commit 2") {
		t.Errorf("Expected first log to contain 'commit 2', got %s", logs[0])
	}
	if !strings.Contains(logs[1], "commit 1") {
		t.Errorf("Expected second log to contain 'commit 1', got %s", logs[1])
	}

	// Test Log empty
	// Create a new empty repo (no commits yet) - actually init creates empty
	emptyDir, _ := os.MkdirTemp("", "git-empty")
	defer os.RemoveAll(emptyDir)
	// We need to init it
	// But Log on empty repo might fail "does not have any commits yet"

	// Let's test non-existent dir
	_, err = c.Log(filepath.Join(localDir, "nonexistent"))
	if err == nil {
		t.Error("Log should fail on non-existent dir")
	}
}

func TestClient_DiffStat(t *testing.T) {
	localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "commit 1")
	sha1, _ := c.CurrentCommitSHA(localDir)

	// Second commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1-modified"), 0644)
	// Add a new file
	os.WriteFile(filepath.Join(localDir, "f2"), []byte("new file"), 0644)
	c.Commit(localDir, "commit 2")
	sha2, _ := c.CurrentCommitSHA(localDir)

	// Test DiffStat
	stat, err := c.DiffStat(localDir, sha1, sha2)
	if err != nil {
		t.Fatalf("DiffStat failed: %v", err)
	}

	// Expected output roughly:
	//  f1 | 2 +-
	//  f2 | 1 +
	//  2 files changed, 2 insertions(+), 1 deletion(-)

	if !strings.Contains(stat, "f1") {
		t.Error("DiffStat missing f1")
	}
	if !strings.Contains(stat, "f2") {
		t.Error("DiffStat missing f2")
	}
	if !strings.Contains(stat, "insertions") {
		t.Error("DiffStat missing insertions count")
	}

	// Test DiffStat with invalid SHAs
	_, err = c.DiffStat(localDir, "invalid", "invalid")
	if err == nil {
		t.Error("DiffStat should fail with invalid SHA")
	}
}

func TestClient_Diff(t *testing.T) {
    localDir, _ := setupTestRepo(t)
	defer os.RemoveAll(localDir)

	c := NewClient()

    // Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("line1\n"), 0644)
	c.Commit(localDir, "commit 1")
	sha1, _ := c.CurrentCommitSHA(localDir)

	// Second commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("line1\nline2\n"), 0644)
	c.Commit(localDir, "commit 2")
	sha2, _ := c.CurrentCommitSHA(localDir)

    diff, err := c.Diff(localDir, sha1, sha2)
    if err != nil {
        t.Fatalf("Diff failed: %v", err)
    }

    if !strings.Contains(diff, "+line2") {
        t.Errorf("Diff should contain +line2, got: %s", diff)
    }

    // Test Error
    _, err = c.Diff(localDir, "badsha", "badsha2")
    if err == nil {
        t.Error("Diff should fail with bad SHAs")
    }
}
