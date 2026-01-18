package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplexityCommand(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()

	// Create a Go file with known complexity
	// ComplexFunc: 1 + 3 ifs = 4
	// SimpleFunc: 1
	code := `package main

func ComplexFunc(n int) {
	if n > 0 {
		// ...
	}
	if n > 10 {
		// ...
	}
	if n > 100 {
		// ...
	}
}

func SimpleFunc() {
	// ...
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte(code), 0644)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Default Threshold", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "complexity", tmpDir, "--threshold", "1")
		if err != nil {
			t.Errorf("Complexity command failed: %v", err)
		}

		if !strings.Contains(output, "ComplexFunc") {
			t.Errorf("Expected output to contain 'ComplexFunc', got %q", output)
		}
		if !strings.Contains(output, "SimpleFunc") {
			t.Errorf("Expected output to contain 'SimpleFunc', got %q", output)
		}
		if !strings.Contains(output, "4") {
			t.Errorf("Expected output to contain complexity '4', got %q", output)
		}
	})

	t.Run("High Threshold", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "complexity", tmpDir, "--threshold", "10")
		if err != nil {
			t.Errorf("Complexity command failed: %v", err)
		}

		if strings.Contains(output, "ComplexFunc") {
			t.Errorf("Did not expect 'ComplexFunc' with threshold 10, got %q", output)
		}
		if !strings.Contains(output, "Good job") {
			t.Errorf("Expected 'Good job' message, got %q", output)
		}
	})

	t.Run("JSON Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "complexity", tmpDir, "--threshold", "1", "--json")
		if err != nil {
			t.Errorf("Complexity command failed: %v", err)
		}

		var results []FunctionComplexity
		if err := json.Unmarshal([]byte(output), &results); err != nil {
			t.Fatalf("Failed to parse JSON output: %v", err)
		}

		found := false
		for _, res := range results {
			if res.Function == "ComplexFunc" && res.Complexity == 4 {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Expected JSON to contain ComplexFunc with complexity 4")
		}
	})

	t.Run("Ignore non-go files", func(t *testing.T) {
		os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("func Fake() {}"), 0644)
		output, err := executeCommand(rootCmd, "complexity", tmpDir, "--threshold", "1")
		if err != nil {
			t.Errorf("Complexity command failed: %v", err)
		}
		if strings.Contains(output, "Fake") {
			t.Errorf("Should not analyze text files")
		}
	})
}
