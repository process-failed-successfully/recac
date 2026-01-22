package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestEvolutionAnalysis(t *testing.T) {
	// 1. Setup Temp Git Repo
	repoDir := t.TempDir()

	// Helper to run git in repoDir
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}

	// Initialize repo
	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")

	// Commit 1: Initial (1 file, 1 func, 0 TODOs)
	mainGo := filepath.Join(repoDir, "main.go")
	err := os.WriteFile(mainGo, []byte(`package main
func main() {
	println("hello")
}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "Initial commit")

	// Sleep to ensure timestamp difference (git log order)
	time.Sleep(1 * time.Second)

	// Commit 2: Add complexity and TODO (1 file, 2 funcs, 1 TODO)
	err = os.WriteFile(mainGo, []byte(`package main
func main() {
	println("hello")
}
func complex() {
	if true {
		// TODO: fix this
		println("complex")
	}
}`), 0644)
	if err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-m", "Add complexity")

	// 2. Run Analysis
	// We want to analyze the repoDir
	// We mocked execCommand in shared_utils.go, but here we want to use the REAL exec.Command
	// because we are interacting with a real (temp) git repo.
	// Since execCommand defaults to exec.Command, we are good unless another test changed it.
	// Safe practice: ensure it is reset.
	execCommand = exec.Command
	defer func() { execCommand = exec.Command }()

	// We pass a dummy command object just for output
	cmd := evolutionCmd
	// Capture output to avoid polluting test logs
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	metrics, err := runEvolutionAnalysis(cmd, repoDir, 30)
	if err != nil {
		t.Fatalf("Evolution analysis failed: %v", err)
	}

	// 3. Verify
	// We expect 2 metrics points (one for each commit)
	if len(metrics) != 2 {
		t.Errorf("Expected 2 metrics points, got %d", len(metrics))
	}

	// Sort by date/commit to ensure order (runEvolutionAnalysis iterates backwards but appends, so [0] is latest)
	// Actually logic says: for i := len(commits) - 1; i >= 0 ...
	// So metrics[0] should be the OLDEST commit (Initial).
	// Let's check the code:
	// commits are returned by `git log` default order (newest first).
	// Loop: `for i := len(commits) - 1; i >= 0` -> Oldest first.
	// So metrics[0] is Commit 1.

	m1 := metrics[0]
	if m1.TODOs != 0 {
		t.Errorf("Commit 1: Expected 0 TODOs, got %d", m1.TODOs)
	}
	if m1.Complexity < 1 { // At least main() has complexity 1
		t.Errorf("Commit 1: Expected complexity >= 1, got %d", m1.Complexity)
	}

	m2 := metrics[1]
	if m2.TODOs != 1 {
		t.Errorf("Commit 2: Expected 1 TODO, got %d", m2.TODOs)
	}
	// func complex has if -> +1, base 1 = 2. main base 1. Total 3?
	// Let's verify complexity calculation.
	if m2.Complexity <= m1.Complexity {
		t.Errorf("Commit 2 should be more complex than Commit 1")
	}
}
