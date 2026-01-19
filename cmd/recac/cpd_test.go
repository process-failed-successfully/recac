package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCPD_RunCPD(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two identical files
	content := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
	fmt.Println("World")
	fmt.Println("This")
	fmt.Println("Is")
	fmt.Println("A")
	fmt.Println("Duplicate")
	fmt.Println("Block")
	fmt.Println("Test")
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte(content), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Min lines = 5
	dups, err := runCPD(tmpDir, 5, nil)
	if err != nil {
		t.Fatalf("runCPD failed: %v", err)
	}

	if len(dups) == 0 {
		t.Fatal("expected duplicates, got 0")
	}

	// Verify we found matches in both files
	foundFile1 := false
	foundFile2 := false
	for _, d := range dups {
		for _, l := range d.Locations {
			if filepath.Base(l.File) == "file1.go" {
				foundFile1 = true
			}
			if filepath.Base(l.File) == "file2.go" {
				foundFile2 = true
			}
		}
	}

	if !foundFile1 || !foundFile2 {
		t.Errorf("expected duplicates in both files, got file1=%v file2=%v", foundFile1, foundFile2)
	}
}

func TestCPD_Merging(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a large identical file content (20 lines)
	var content strings.Builder
	for i := 0; i < 20; i++ {
		content.WriteString(fmt.Sprintf("line %d\n", i))
	}

	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte(content.String()), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte(content.String()), 0644)

	// Min lines = 5.
	// We expect ONE duplicate block of length 20.
	// Without merging, we would get ~15 overlapping blocks.

	dups, err := runCPD(tmpDir, 5, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(dups) != 1 {
		t.Errorf("Expected 1 duplicate block after merging, got %d", len(dups))
		for i, d := range dups {
			t.Logf("Dup %d: %d lines", i, d.LineCount)
		}
	} else {
		if dups[0].LineCount != 20 {
			t.Errorf("Expected 20 lines, got %d", dups[0].LineCount)
		}
	}
}

func TestCPD_Ignore(t *testing.T) {
	tmpDir := t.TempDir()
	content := `line1
line2
line3
line4
line5
`
	// Create duplicate files
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte(content), 0644)
	os.WriteFile(filepath.Join(tmpDir, "ignored.go"), []byte(content), 0644)

	dups, err := runCPD(tmpDir, 5, []string{"ignored.go"})
	if err != nil {
		t.Fatalf("runCPD failed: %v", err)
	}

	if len(dups) != 0 {
		t.Errorf("expected 0 duplicates when one file is ignored, got %d", len(dups))
	}
}

func TestCPD_Command(t *testing.T) {
	tmpDir := t.TempDir()
	content := `line1
line2
line3
line4
line5
`
	// Create duplicate files
	os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte(content), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte(content), 0644)

	// Test 1: JSON output (default ignore)
    // Create a fresh command for each execution
	cmd := newCPDCmd()
	output, err := executeCommand(cmd, tmpDir, "--min-lines", "5", "--json")
	if err != nil {
		t.Fatalf("cpd command failed: %v", err)
	}

	var dups []Duplication
	if err := json.Unmarshal([]byte(output), &dups); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

	if len(dups) == 0 {
		t.Fatal("expected duplicates in JSON output")
	}

    // Test 2: With Ignore flag
    // Create a file that should be ignored
    os.WriteFile(filepath.Join(tmpDir, "ignore_me.go"), []byte(content), 0644)

    // Run with ignore
	cmd = newCPDCmd()
    output, err = executeCommand(cmd, tmpDir, "--min-lines", "5", "--json", "--ignore", "ignore_me.go")
    if err != nil {
        t.Fatalf("cpd command failed: %v", err)
    }
     if err := json.Unmarshal([]byte(output), &dups); err != nil {
		t.Fatalf("failed to parse JSON: %v\nOutput: %s", err, output)
	}

    // Check if ignore_me.go is in results
    for _, d := range dups {
        for _, l := range d.Locations {
            if filepath.Base(l.File) == "ignore_me.go" {
                t.Error("found ignore_me.go despite --ignore flag")
            }
        }
    }

	// Test 3: Fail Flag
	cmd = newCPDCmd()
	_, err = executeCommand(cmd, tmpDir, "--min-lines", "5", "--fail")
	if err == nil {
		t.Error("expected error with --fail flag when duplicates exist")
	} else {
		if err.Error() == "" {
			t.Error("got empty error message")
		}
	}
}
