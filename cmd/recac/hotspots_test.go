package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupGitRepo(t *testing.T, dir string) {
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	run("config", "commit.gpgsign", "false")
}

func commitFile(t *testing.T, dir, filename, content string) {
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}
	run("add", filename)
	run("commit", "-m", fmt.Sprintf("update %s", filename))
}

func TestGetGitChurn(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create file A (Churn=1)
	commitFile(t, tmpDir, "a.go", "package main\n")

	// Create file B (Churn=1)
	commitFile(t, tmpDir, "b.go", "package main\n")

	// Modify file A (Churn=2)
	commitFile(t, tmpDir, "a.go", "package main\nfunc main() {}\n")

	// Verify
	churn, err := getGitChurn(tmpDir, 30)
	if err != nil {
		t.Fatalf("getGitChurn failed: %v", err)
	}

	if churn["a.go"] != 2 {
		t.Errorf("expected churn 2 for a.go, got %d", churn["a.go"])
	}
	if churn["b.go"] != 1 {
		t.Errorf("expected churn 1 for b.go, got %d", churn["b.go"])
	}
}

func TestRunHotspotAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// File A: High Complexity (Loop + If)
	complexCode := `package main
	func complex() {
		for i := 0; i < 10; i++ {
			if i % 2 == 0 {
				print(i)
			}
		}
	}`
	// Complexity: 1 (base) + 1 (for) + 1 (if) = 3

	// File B: Low Complexity
	simpleCode := `package main
	func simple() {
		print("hello")
	}`
	// Complexity: 1

	// Create files
	commitFile(t, tmpDir, "complex.go", complexCode)
	commitFile(t, tmpDir, "simple.go", simpleCode)

	// Modify complex.go to increase churn (Churn=2)
	commitFile(t, tmpDir, "complex.go", complexCode+"\n// mod")

	// Analyze
	hotspots, err := runHotspotAnalysis(tmpDir, 30)
	if err != nil {
		t.Fatalf("runHotspotAnalysis failed: %v", err)
	}

	// Verify
	// complex.go: Churn=2, Complexity=3, Score=6
	// simple.go: Churn=1, Complexity=1, Score=1

	foundComplex := false
	foundSimple := false

	for _, h := range hotspots {
		if h.File == "complex.go" {
			foundComplex = true
			if h.Score != 6 {
				t.Errorf("expected score 6 for complex.go, got %f (Churn=%d, Comp=%d)", h.Score, h.Churn, h.Complexity)
			}
		}
		if h.File == "simple.go" {
			foundSimple = true
			if h.Score != 1 {
				t.Errorf("expected score 1 for simple.go, got %f (Churn=%d, Comp=%d)", h.Score, h.Churn, h.Complexity)
			}
		}
	}

	if !foundComplex {
		t.Error("complex.go not found in hotspots")
	}
	if !foundSimple {
		t.Error("simple.go not found in hotspots")
	}
}

func TestHotspotsCmd(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create files
	complexCode := `package main
func Complex() {
    if true { if true { } }
}`
	commitFile(t, tmpDir, "complex.go", complexCode)

	// Run command
	// We need to pass the directory as argument.
	// Also capture output.

	cmd := hotspotsCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Reset flags?
	hotspotsDays = 30
	hotspotsLimit = 10

	// Execute
	// The command implementation:
	// RunE: func(cmd *cobra.Command, args []string) error {
	//    path := "."
	//    if len(args) > 0 { path = args[0] }
	//    results, err := runHotspotAnalysis(path, hotspotsDays)
	//    ...
	// }

	if err := cmd.RunE(cmd, []string{tmpDir}); err != nil {
		t.Fatalf("cmd.RunE failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "complex.go") {
		t.Errorf("Output should contain complex.go, got:\n%s", output)
	}
	if !strings.Contains(output, "HOTSPOTS REPORT") {
		t.Error("Output should contain report header")
	}
}
