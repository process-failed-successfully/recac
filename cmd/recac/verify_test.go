package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCmd_Integration(t *testing.T) {
	// Create a temp directory for the git repo
	tempDir := t.TempDir()

	// Helper to run git commands
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = tempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
	}

	// Initialize git repo
	git("init")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test User")

	// 1. Create a clean Go file (Baseline)
	cleanCode := `package main

func Simple() {
	println("Hello")
}
`
	filePath := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(filePath, []byte(cleanCode), 0644); err != nil {
		t.Fatal(err)
	}

	git("add", "main.go")
	git("commit", "-m", "Initial commit")

	// 2. Modify the file: Add Complexity and Security Issue on NEW lines
	// We keep Simple() intact (line 3-5).
	// We add complex function at the end.
	dirtyCode := `package main

import "fmt"

func Simple() {
	println("Hello")
}

// Added complexity
func Complex(n int) {
	if n > 0 {
		if n > 1 {
			if n > 2 {
				if n > 3 {
					if n > 4 {
						if n > 5 {
							if n > 6 {
								if n > 7 {
									if n > 8 {
										if n > 9 {
											if n > 10 {
												if n > 11 {
													if n > 12 {
														if n > 13 {
															if n > 14 {
																fmt.Println("Too complex")
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
}

// Added security issue
func Secret() {
	key := "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"
	fmt.Println(key)
}
`
	if err := os.WriteFile(filePath, []byte(dirtyCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Setup the command
	// We need to run the verify command inside the tempDir.
	// Since verifyCmd uses os.Getwd() implicitly via git commands or file paths?
	// `runVerify` calls `getChangedFiles` which calls `git diff`.
	// `exec.Command("git")` uses the current working directory of the process by default.
	// So we must change directory to tempDir.

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}

	// Run Verify
	// It should detect issues because they are in changed lines (unstaged diff).
	// We need to capture output.
	// Since `verifyCmd` writes to `cmd.OutOrStdout()`, we can capture it.

	// Reset flags using Cobra methods to ensure consistency
	verifyCmd.Flags().Set("staged", "false")
	verifyCmd.Flags().Set("all", "false")
	verifyCmd.Flags().Set("json", "true")

	// Helper to execute verify
	executeVerify := func() ([]VerifyIssue, error) {
		buf := new(bytes.Buffer)
		verifyCmd.SetOut(buf)

		// Ensure JSON is strictly true for test parsing, unless overridden?
		// We'll rely on test cases setting flags correctly.
		// verifyCmd.Flags().Set("json", "true")

		err := runVerify(verifyCmd, nil)

		var issues []VerifyIssue
		if buf.Len() > 0 {
			// It might output "[]" if empty
			if jsonErr := json.Unmarshal(buf.Bytes(), &issues); jsonErr != nil {
				return nil, fmt.Errorf("failed to unmarshal json: %v. Output: %s", jsonErr, buf.String())
			}
		}
		return issues, err
	}

	// Test Case 1: Unstaged changes (should find issues)
	issues, err := executeVerify()
	// Note: JSON mode does not return error even if issues found

	if len(issues) < 2 {
		t.Errorf("Expected at least 2 issues (Complexity, Security), got %d", len(issues))
	}

	foundComplexity := false
	foundSecurity := false
	for _, i := range issues {
		if i.Type == "Complexity" {
			foundComplexity = true
			if !strings.Contains(i.File, "main.go") {
				t.Errorf("Complexity issue file mismatch: %s", i.File)
			}
		}
		if i.Type == "Security" {
			foundSecurity = true
			if !strings.Contains(i.File, "main.go") {
				t.Errorf("Security issue file mismatch: %s", i.File)
			}
		}
	}

	if !foundComplexity {
		t.Error("Did not find expected Complexity issue")
	}
	if !foundSecurity {
		t.Error("Did not find expected Security issue")
	}

	// Test Case 2: Staged changes
	// Stage the file
	git("add", "main.go")

	// Set staged flag
	verifyCmd.Flags().Set("staged", "true")

	issues, err = executeVerify()
	if len(issues) < 2 {
		t.Errorf("Expected issues in staged changes, got %d", len(issues))
	}

	// Test Case 3: Commit the changes. verify should find nothing (because no diff).
	git("commit", "-m", "Bad commit")

	verifyCmd.Flags().Set("staged", "false")
	issues, err = executeVerify()
	if err != nil {
		t.Errorf("Expected no error for clean state, got: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for clean state, got %d", len(issues))
	}

	// Test Case 4: Verify All (should find issues even if committed, if we pass file arg or just --all on changed? Wait.)
	// `verify --all` only checks *changed* files if no args provided.
	// Since we just committed, there are no changed files.
	// So `verify --all` should return nothing.

	verifyCmd.Flags().Set("all", "true")
	issues, err = executeVerify()
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues because no files changed, got %d", len(issues))
	}

	// Test Case 5: Verify explicit file (with --all implicitly? No, check logic.)
	// Logic: if args provided, use them.
	// Logic: if verifyAll is true, check whole file.
	// Logic: if verifyAll is false, check changed lines.
	// Since we committed, git diff is empty. So changed lines are nil/empty.
	// If changed lines are empty, `isLineInIntervals` returns false.
	// So explicit file verification without --all on a clean file (git-wise) should return NOTHING.

	verifyCmd.Flags().Set("staged", "false")
	verifyCmd.Flags().Set("all", "false")
	// We need to pass args. `runVerify` takes `args`.
	// But `runVerify` uses `args` from function argument.

	// Helper for args
	executeVerifyWithArgs := func(args []string) ([]VerifyIssue, error) {
		buf := new(bytes.Buffer)
		verifyCmd.SetOut(buf)
		err := runVerify(verifyCmd, args)
		var issues []VerifyIssue
		if buf.Len() > 0 {
			json.Unmarshal(buf.Bytes(), &issues)
		}
		return issues, err
	}

	issues, err = executeVerifyWithArgs([]string{"main.go"})
	// Should be 0 because no diff lines.
	if len(issues) != 0 {
		t.Errorf("Expected 0 issues for clean file without --all, got %d", len(issues))
	}

	// Test Case 6: Verify explicit file WITH --all
	verifyCmd.Flags().Set("all", "true")
	issues, err = executeVerifyWithArgs([]string{"main.go"})
	// Should find issues because we scan whole file.
	if len(issues) < 2 {
		t.Errorf("Expected issues with --all on explicit file, got %d", len(issues))
	}
}

func TestParseDiffHunks(t *testing.T) {
	diff := `diff --git a/main.go b/main.go
index e69de29..d95f3ad 100644
--- a/main.go
+++ b/main.go
@@ -1,2 +3,4 @@
 func One() {}
+func Two() {}
+func Three() {}
 func Four() {}
@@ -10,0 +15,2 @@
+func New() {}
+func New2() {}
`
	intervals := parseDiffHunks(diff)

	expected := []LineInterval{
		{3, 6},   // +3,4 -> 3, 4, 5, 6 (start at 3, count 4)
		{15, 16}, // +15,2 -> 15, 16
	}

	if len(intervals) != 2 {
		t.Fatalf("Expected 2 intervals, got %d", len(intervals))
	}

	if intervals[0].Start != 3 || intervals[0].End != 6 {
		t.Errorf("Interval 1 mismatch: got %v, want %v", intervals[0], expected[0])
	}
	if intervals[1].Start != 15 || intervals[1].End != 16 {
		t.Errorf("Interval 2 mismatch: got %v, want %v", intervals[1], expected[1])
	}
}
