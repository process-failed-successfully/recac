package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"recac/internal/mutation"
	"strings"
	"testing"
)

// TestMutationCmdIntegration tests the mutation command end-to-end using a dummy package.
func TestMutationCmdIntegration(t *testing.T) {
	// 1. Create a dummy package in a temporary directory
	tmpDir, err := os.MkdirTemp("", "mutation-test-pkg-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	goMod := `module mutationtest
go 1.22
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create main.go with logic to mutate
	mainGo := `package main

func Add(a, b int) int {
	return a + b
}

func IsPositive(n int) bool {
	if n > 0 {
		return true
	}
	return false
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Create main_test.go
	// Case 1: Add is tested, IsPositive is tested but weakly (mutant might survive)
	testGo := `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("Add(2,3) should be 5")
	}
}

func TestIsPositive(t *testing.T) {
	// Weak test: only checking positive case, so "n > 0" mutated to "n >= 0" for input 0 might survive if we don't test 0.
	// Actually, if we mutate > to >=.
	// If we pass 1, 1>0 is true, 1>=0 is true. Mutant survives.
	if !IsPositive(1) {
		t.Error("1 should be positive")
	}
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(testGo), 0644); err != nil {
		t.Fatalf("failed to write main_test.go: %v", err)
	}

	// 2. Prepare the command
	// We use the package variable `mutationCmd` directly.
	// We must ensure flags are set correctly.

	// Save original flags
	origVerbose := mutationVerbose
	origDryRun := mutationDryRun
	defer func() {
		mutationVerbose = origVerbose
		mutationDryRun = origDryRun
	}()

	mutationVerbose = true
	mutationDryRun = false // We want to run tests

	// Mock stdout/stderr
	cmd := mutationCmd
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// 3. Execute
	// We pass the tmpDir as the argument
	// We call runMutation directly instead of cmd.Execute() to avoid Cobra root command issues in limited context
	err = runMutation(cmd, []string{tmpDir})

	// 4. Assertions
	output := outBuf.String()
	t.Logf("Mutation Output:\n%s", output)

	// Since we expect some mutants to survive (IsPositive), the command should return an error.
	if err == nil {
		t.Error("Expected error due to surviving mutants, but got nil")
	} else if !strings.Contains(err.Error(), "mutation score below 100%") {
		t.Errorf("Expected 'mutation score below 100%%' error, got: %v", err)
	}

	if !strings.Contains(output, "Mutant Survived") {
		t.Error("Expected output to report surviving mutants")
	}

	if !strings.Contains(output, "Killed") {
		t.Error("Expected output to report killed mutants (Add function)")
	}
}

func TestMutationGeneration(t *testing.T) {
	code := `package main
func foo(a int) bool {
	return a > 10 && a < 20
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	mutator := mutation.NewMutator(fset)
	mutations := mutator.GenerateMutations(file, "test.go")

	if len(mutations) == 0 {
		t.Fatal("Expected mutations, found none")
	}

	foundGT := false
	foundLT := false
	foundAND := false

	for _, m := range mutations {
		if m.Original == ">" && m.Mutated == "<=" {
			foundGT = true
		}
		if m.Original == "<" && m.Mutated == ">=" {
			foundLT = true
		}
		if m.Original == "&&" && m.Mutated == "||" {
			foundAND = true
		}
	}

	if !foundGT {
		t.Error("Did not find > mutation")
	}
	if !foundLT {
		t.Error("Did not find < mutation")
	}
	if !foundAND {
		t.Error("Did not find && mutation")
	}
}
