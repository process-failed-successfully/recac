package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchitectVisualizeCmd_Run(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-architect-visualize-test-run-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	archContent := `
system_name: "Test System"
version: "1.0"
components:
  - id: "Service A"
    type: "service"
`
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	if err := os.WriteFile(archFile, []byte(archContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Reset global state
	visualizeOut = ""
	// Reset flags
	architectVisualizeCmd.Flags().Set("out", "")

	// Execute via rootCmd
	output := captureOutput(func() {
		rootCmd.SetArgs([]string{"architect", "visualize", archFile})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Command failed: %v", err)
		}
	})

	if !strings.Contains(output, "graph TD") {
		t.Errorf("Expected 'graph TD', got:\n%s", output)
	}
	if !strings.Contains(output, "Service_A[\"Service A\"]") {
		t.Errorf("Expected Service A node, got:\n%s", output)
	}
}

func TestArchitectVisualizeCmd_ToFile(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-architect-visualize-test-file-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	archContent := `
system_name: "Test System"
version: "1.0"
components:
  - id: "DB"
    type: "database"
`
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	if err := os.WriteFile(archFile, []byte(archContent), 0644); err != nil {
		t.Fatal(err)
	}

	outFile := filepath.Join(tmpDir, "output.mmd")

	// Reset global state
	visualizeOut = ""
	architectVisualizeCmd.Flags().Set("out", "")

	// Execute via rootCmd
	output := captureOutput(func() {
		rootCmd.SetArgs([]string{"architect", "visualize", archFile, "--out", outFile})
		if err := rootCmd.Execute(); err != nil {
			t.Errorf("Command failed: %v", err)
		}
	})

	// Check if file exists
	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("Output file not created: %s", outFile)
	}

	if !strings.Contains(string(content), "DB[(\"DB\")]") {
		t.Errorf("File content mismatch, got:\n%s", string(content))
	}

	if !strings.Contains(output, "Mermaid diagram written to") {
		t.Errorf("Expected success message, got: %s", output)
	}
}
