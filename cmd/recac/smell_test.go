package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSmellCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go file with various smells
	code := `package main

// Long function (LOC > 50 if we pad it)
// Many params (> 5)
// Many returns (> 3)
// High complexity (> 10)
// Deep nesting (> 4)

func SmellyFunc(a, b, c, d, e, f int) (int, int, int, int) {
	// Deep nesting
	if a > 0 {
		if b > 0 {
			if c > 0 {
				if d > 0 {
					if e > 0 {
						return 1, 2, 3, 4
					}
				}
			}
		}
	}

	// Pad lines
	x := 0
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	x++
	return 0, 0, 0, 0
}

func CleanFunc() {
	// ...
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "smell.go"), []byte(code), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Detect Smells", func(t *testing.T) {
		// Set low thresholds to ensure we trigger detections
		output, err := executeCommand(rootCmd, "smell", tmpDir, "--loc", "10", "--nesting", "2", "--params", "2", "--returns", "2")
		if err != nil {
			t.Errorf("Smell command failed: %v", err)
		}

		if !strings.Contains(output, "Long Function") {
			t.Error("Expected to find Long Function")
		}
		if !strings.Contains(output, "Many Parameters") {
			t.Error("Expected to find Many Parameters")
		}
		if !strings.Contains(output, "Many Returns") {
			t.Error("Expected to find Many Returns")
		}
		if !strings.Contains(output, "Deep Nesting") {
			t.Error("Expected to find Deep Nesting")
		}
		if !strings.Contains(output, "SmellyFunc") {
			t.Error("Expected to find SmellyFunc")
		}
	})

	t.Run("JSON Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "smell", tmpDir, "--loc", "10", "--json")
		if err != nil {
			t.Errorf("Smell command failed: %v", err)
		}

		var findings []SmellFinding
		if err := json.Unmarshal([]byte(output), &findings); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}

		found := false
		for _, f := range findings {
			if f.Function == "SmellyFunc" && f.Type == "Long Function" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected JSON finding for SmellyFunc/Long Function")
		}
	})

	t.Run("Clean Code", func(t *testing.T) {
		// Use very high thresholds
		output, err := executeCommand(rootCmd, "smell", tmpDir, "--loc", "1000", "--nesting", "10", "--params", "10", "--returns", "10")
		if err != nil {
			t.Errorf("Smell command failed: %v", err)
		}
		if !strings.Contains(output, "Clean code") {
			t.Errorf("Expected 'Clean code', got: %s", output)
		}
	})
}
