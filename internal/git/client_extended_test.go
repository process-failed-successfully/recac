package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestClient_ExtendedOperations(t *testing.T) {
	localDir, remoteDir := setupTestRepo(t)
	defer os.RemoveAll(localDir)
	defer os.RemoveAll(remoteDir)

	c := NewClient()

	// Initial commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v1"), 0644)
	c.Commit(localDir, "init")

	// Get SHA
	v1SHA, err := c.CurrentCommitSHA(localDir)
	if err != nil {
		t.Fatalf("CurrentCommitSHA failed: %v", err)
	}

	// Second commit
	os.WriteFile(filepath.Join(localDir, "f1"), []byte("v2"), 0644)
	c.Commit(localDir, "update v2")

	v2SHA, err := c.CurrentCommitSHA(localDir)
	if err != nil {
		t.Fatalf("CurrentCommitSHA failed: %v", err)
	}

	// 1. Test Diff
	diff, err := c.Diff(localDir, v1SHA, v2SHA)
	if err != nil {
		t.Errorf("Diff failed: %v", err)
	}
	if !strings.Contains(diff, "v2") {
		t.Errorf("Diff expected to contain v2, got: %s", diff)
	}

	// 2. Test DiffStat
	stat, err := c.DiffStat(localDir, v1SHA, v2SHA)
	if err != nil {
		t.Errorf("DiffStat failed: %v", err)
	}
	if !strings.Contains(stat, "f1") {
		t.Errorf("DiffStat expected to contain f1, got: %s", stat)
	}

	// 3. Test Log
	logs, err := c.Log(localDir, "-n", "2")
	if err != nil {
		t.Errorf("Log failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("Log returned empty")
	}

	// 4. Test Tagging
	if err := c.Tag(localDir, "v0.1.0"); err != nil {
		t.Errorf("Tag failed: %v", err)
	}

	latest, err := c.LatestTag(localDir)
	if err != nil {
		t.Errorf("LatestTag failed: %v", err)
	}
	if latest != "v0.1.0" {
		t.Errorf("Expected tag v0.1.0, got %s", latest)
	}

	// Push tags
	if err := c.PushTags(localDir); err != nil {
		t.Errorf("PushTags failed: %v", err)
	}

	// Check remote tags
	cmd := exec.Command("git", "-C", remoteDir, "tag")
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "v0.1.0") {
		t.Error("Remote missing tag v0.1.0")
	}

	// Delete Tag
	if err := c.DeleteTag(localDir, "v0.1.0"); err != nil {
		t.Errorf("DeleteTag failed: %v", err)
	}
	// Verify deletion
	latest, _ = c.LatestTag(localDir)
	if latest != "" {
		t.Errorf("Expected no tags, got %s", latest)
	}

	// 5. Test Bisect
	// Need at least one more commit to bisect effectively?
	// v1 (good), v2 (bad)

	if err := c.BisectStart(localDir, v2SHA, v1SHA); err != nil {
		t.Errorf("BisectStart failed: %v", err)
	}

	bisectLog, err := c.BisectLog(localDir)
	if err != nil {
		t.Errorf("BisectLog failed: %v", err)
	}
	if len(bisectLog) == 0 {
		t.Error("BisectLog returned empty")
	}

	// Mark bad/good
	// Since v2 is bad and v1 is good, and they are adjacent, bisect might be done immediately or waiting.
	// Let's just test the commands run without error.
	if err := c.BisectBad(localDir, ""); err != nil {
		t.Errorf("BisectBad failed: %v", err)
	}

	// Reset
	if err := c.BisectReset(localDir); err != nil {
		t.Errorf("BisectReset failed: %v", err)
	}

	// 6. Test Recover
	// Create a lock file
	lockFile := filepath.Join(localDir, ".git", "index.lock")
	os.WriteFile(lockFile, []byte("lock"), 0644)

	if err := c.Recover(localDir); err != nil {
		t.Errorf("Recover failed: %v", err)
	}

	if _, err := os.Stat(lockFile); !os.IsNotExist(err) {
		t.Error("Recover failed to remove index.lock")
	}
}

func TestClient_GlobalConfig(t *testing.T) {
	// Global config modifies user's global .gitconfig, which is dangerous in a shared env or dev machine.
	// In this sandbox, it might be okay, but we should probably use a custom HOME or skip it if we want to be safe.
	// However, usually CI environments are ephemeral.
	// Let's set a custom HOME to test this safely.

	tempHome, err := os.MkdirTemp("", "git-home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	// Mock HOME env var for the test process?
	// The `c.runWithMasking` uses `os.Environ()` which inherits current process env.
	// We can't easily change `os.Environ()` for the subprocess without changing the Client code or mocking exec.
	// But `exec.Command` uses `os.Environ()` by default only if `cmd.Env` is nil.
	// The `Client.runWithMasking` explicitly appends to `os.Environ()`.

	// So we need to set os.Setenv for the current process, which affects all tests running in parallel.
	// If tests are not parallel, it's fine. `setupTestRepo` doesn't seem to set Parallel.

	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempHome)

	c := NewClient()

	if err := c.ConfigGlobal("user.name", "Global User"); err != nil {
		t.Errorf("ConfigGlobal failed: %v", err)
	}

	// Check config
	// We need to run git config --global list
	// But our `Client` doesn't have a generic `ConfigGet`.
	// Let's verify via file existence or exec.

	cmd := exec.Command("git", "config", "--global", "user.name")
	// Make sure this command also uses the new HOME
	cmd.Env = append(os.Environ(), "HOME="+tempHome)
	out, err := cmd.Output()
	if err != nil {
		t.Errorf("Failed to read global config: %v", err)
	}
	if strings.TrimSpace(string(out)) != "Global User" {
		t.Errorf("Expected Global User, got %s", string(out))
	}

	// Test ConfigAddGlobal
	if err := c.ConfigAddGlobal("user.email", "global@example.com"); err != nil {
		t.Errorf("ConfigAddGlobal failed: %v", err)
	}
}
