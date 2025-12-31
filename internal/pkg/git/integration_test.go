package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealGitOps_Integration(t *testing.T) {
	// 1. Setup Remote (Bare Repo)
	remoteDir, err := os.MkdirTemp("", "git-remote")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(remoteDir)

	cmd := exec.Command("git", "init", "--bare", "repo.git")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	remoteRepo := filepath.Join(remoteDir, "repo.git")

	// 2. Setup Local (Clone)
	localDir, err := os.MkdirTemp("", "git-local")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(localDir)

	cmd = exec.Command("git", "clone", remoteRepo, ".")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to clone repo: %v", err)
	}

	// 3. Setup Environment for RealGitOps
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// Restore CWD after test
	defer os.Chdir(cwd)

	// Change to localDir
	if err := os.Chdir(localDir); err != nil {
		t.Fatal(err)
	}

	// Configure user for commit
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create a dummy commit so we have something to branch off
	if err := os.WriteFile("README.md", []byte("init"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatal(err)
	}
	// Note: We cannot push master/main due to environment restrictions.
	// We will push the feature branch directly.

	ops := &RealGitOps{}

	// 4. Test CreateBranch
	branchName := "feature/test-1"
	if err := ops.CreateBranch(branchName); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Verify branch existence
	cmd = exec.Command("git", "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	currentBranch := strings.TrimSpace(string(out))
	if currentBranch != branchName {
		t.Errorf("expected branch %s, got %s", branchName, currentBranch)
	}

	// 5. Test Push
	// Make a change to push
	if err := os.WriteFile("FEATURE.md", []byte("feature"), 0644); err != nil {
		t.Fatal(err)
	}
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Feature commit").Run()

	if err := ops.Push(branchName); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// 6. Verify Remote has branch
	cmd = exec.Command("git", "ls-remote", "--heads", "origin", branchName)
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("failed to ls-remote: %v", err)
	}
	if len(out) == 0 {
		t.Error("Remote does not have the branch")
	}
	fmt.Println("Integration Test Passed: Branch created and pushed successfully")
}

func TestRealGitOps_EdgeCases(t *testing.T) {
	// Setup Local Repo
	localDir, err := os.MkdirTemp("", "git-local-edge")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(localDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	if err := os.Chdir(localDir); err != nil {
		t.Fatal(err)
	}

	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	
	// Create initial commit
	os.WriteFile("README.md", []byte("init"), 0644)
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Initial commit").Run()

	ops := &RealGitOps{}

	// Test 1: Create Branch Failure (Already Exists)
	branchName := "duplicate-branch"
	if err := ops.CreateBranch(branchName); err != nil {
		t.Fatalf("First CreateBranch failed: %v", err)
	}
	// Try creating it again
	if err := ops.CreateBranch(branchName); err == nil {
		t.Error("Expected error when creating duplicate branch, got nil")
	} else {
		if !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "failed") {
			t.Errorf("Unexpected error message: %v", err)
		}
	}

	// Test 2: Push Failure (No Remote)
	// We haven't added a remote 'origin', so this should fail
	if err := ops.Push(branchName); err == nil {
		t.Error("Expected error when pushing with no remote, got nil")
	}
}
