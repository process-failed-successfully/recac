package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func TestRunHotspotsCmd(t *testing.T) {
	tmpDir := t.TempDir()
	setupGitRepo(t, tmpDir)

	// Create file A: High Complexity
	complexCode := `package main
	func complex() {
		for i := 0; i < 10; i++ {
			if i % 2 == 0 {
				print(i)
			}
		}
	}`
	commitFile(t, tmpDir, "complex.go", complexCode)

	// Mock Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// We need to call RunE of hotspotsCmd but point it to tmpDir.
	// hotspotsCmd.RunE takes args. Arg[0] is path.
	// But hotspotsCmd is a global variable.
	// We can manually invoke the function assigned to RunE if we had access to it by name,
	// or we can use hotspotsCmd.RunE(cmd, []string{tmpDir}).

	// We also need to set flags.
	// hotspotsDays, hotspotsLimit, hotspotsJSON are global vars.
	oldDays := hotspotsDays
	oldLimit := hotspotsLimit
	oldJSON := hotspotsJSON
	defer func() {
		hotspotsDays = oldDays
		hotspotsLimit = oldLimit
		hotspotsJSON = oldJSON
	}()
	hotspotsDays = 30
	hotspotsLimit = 10
	hotspotsJSON = false

	if err := hotspotsCmd.RunE(cmd, []string{tmpDir}); err != nil {
		t.Fatalf("hotspotsCmd failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "HOTSPOTS REPORT") {
		t.Error("Output should contain HOTSPOTS REPORT")
	}
	if !strings.Contains(output, "complex.go") {
		t.Error("Output should contain complex.go")
	}

	// Test JSON
	buf.Reset()
	hotspotsJSON = true
	if err := hotspotsCmd.RunE(cmd, []string{tmpDir}); err != nil {
		t.Fatalf("hotspotsCmd (json) failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "\"file\": \"complex.go\"") {
		t.Error("JSON output should contain complex.go")
	}
}
