package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCallGraph(t *testing.T) {
	// 1. Setup temporary directory with Go files
	tmpDir := t.TempDir()

	fileA := `package main

func funcA() {
	funcB()
}

func funcB() {
	funcC()
}

func funcC() {
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(fileA), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// 2. Test runCallGraph
	tests := []struct {
		name    string
		args    []string
		wantOut []string // Substrings expected in output
		wantErr bool
	}{
		{
			name:    "Default",
			args:    []string{"--dir", tmpDir},
			wantOut: []string{"graph LR", "main_funcA --> main_funcB", "main_funcB --> main_funcC"},
		},
		{
			name:    "Focus funcA",
			args:    []string{"--dir", tmpDir, "--focus", "funcA"},
			wantOut: []string{"graph LR", "main_funcA --> main_funcB"},
		},
		{
			name:    "Invalid Dir",
			args:    []string{"--dir", "/invalid/dir/path"},
			wantErr: true,
		},
		{
			name:    "Current Directory Default",
			args:    []string{}, // No dir arg, defaults to .
			wantOut: []string{"graph LR", "main_funcA --> main_funcB"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "Current Directory Default" {
				cwd, err := os.Getwd()
				if err != nil {
					t.Fatal(err)
				}
				defer os.Chdir(cwd)
				if err := os.Chdir(tmpDir); err != nil {
					t.Fatal(err)
				}
			}

			// Reset globals
			callGraphDir = "."
			callGraphFocus = ""

			// Bind flags to globals using a dummy command to simulate parsing
			cmd := &cobra.Command{Use: "test"}
			cmd.Flags().StringVar(&callGraphDir, "dir", ".", "")
			cmd.Flags().StringVar(&callGraphFocus, "focus", "", "")

			// Parse flags which updates globals
			if err := cmd.Flags().Parse(tt.args); err != nil {
				t.Fatalf("failed to parse flags: %v", err)
			}

			buf := new(bytes.Buffer)
			// Mock the command output
			testCmd := &cobra.Command{}
			testCmd.SetOut(buf)

			err := runCallGraph(testCmd, []string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("runCallGraph() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := buf.String()
			for _, want := range tt.wantOut {
				if !strings.Contains(output, want) {
					t.Errorf("runCallGraph() output missing %q, got:\n%s", want, output)
				}
			}
		})
	}
}

func TestGenerateMermaidCallGraph(t *testing.T) {
	// This is covered by integration test above, but we can verify specific formatting if needed.
	// We'll trust integration test for now.
}
