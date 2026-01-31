package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestPolicy_Check(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()

	// 1. Create a large file
	largeFile := filepath.Join(tmpDir, "large.txt")
	f1, _ := os.Create(largeFile)
	for i := 0; i < 15; i++ {
		f1.WriteString("line\n")
	}
	f1.Close()

	// 2. Create a file with banned content
	badContentFile := filepath.Join(tmpDir, "secret.txt")
	os.WriteFile(badContentFile, []byte("this is a password123\n"), 0644)

	// 3. Create a Go file with banned import
	goFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(goFile, []byte(`package main

import (
	"fmt"
	"unsafe"
)

func main() {
	fmt.Println("hello")
}
`), 0644)

	// Define Policy
	p := Policy{
		Rules: []Rule{
			{Type: RuleFileSize, MaxLines: 10, Message: "Too big"},
			{Type: RuleBannedContent, Pattern: "password", Message: "No secrets", compiledRegex: regexp.MustCompile("password")},
			{Type: RuleBannedImport, Pattern: "unsafe", Message: "No unsafe", compiledRegex: regexp.MustCompile("unsafe")},
		},
	}

	// Run Check
	violations, err := p.Check(tmpDir)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	// Verify violations
	// We expect 3 violations
	if len(violations) != 3 {
		t.Errorf("Expected 3 violations, got %d", len(violations))
		for _, v := range violations {
			t.Logf("Violation: %s - %s", v.File, v.Message)
		}
	}

	// Check details
	foundLarge := false
	foundSecret := false
	foundUnsafe := false

	for _, v := range violations {
		if v.Message == "Too big" {
			foundLarge = true
		}
		if v.Message == "No secrets" {
			foundSecret = true
		}
		if v.Message == "No unsafe" {
			foundUnsafe = true
		}
	}

	if !foundLarge {
		t.Error("Missing file size violation")
	}
	if !foundSecret {
		t.Error("Missing banned content violation")
	}
	if !foundUnsafe {
		t.Error("Missing banned import violation")
	}
}

func TestLoadPolicy(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "policy.yaml")
	content := `
rules:
  - type: banned_import
    pattern: "unsafe"
`
	os.WriteFile(tmpFile, []byte(content), 0644)

	p, err := LoadPolicy(tmpFile)
	if err != nil {
		t.Fatalf("LoadPolicy failed: %v", err)
	}

	if len(p.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(p.Rules))
	}
	if p.Rules[0].Type != RuleBannedImport {
		t.Errorf("Expected RuleBannedImport, got %s", p.Rules[0].Type)
	}
	if p.Rules[0].compiledRegex == nil {
		t.Error("Expected compiledRegex to be set")
	}
}
