package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
)

// MockAgentForRefactor implements agent.Agent interface
type MockAgentForRefactor struct {
	Response string
}

func (m *MockAgentForRefactor) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForRefactor) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestRefactorCmd(t *testing.T) {
	// Setup temporary directory
	tempDir := t.TempDir()
	file1Path := filepath.Join(tempDir, "file1.go")
	file2Path := filepath.Join(tempDir, "file2.go")

	err := os.WriteFile(file1Path, []byte("package main\nfunc Foo() {}"), 0644)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(file2Path, []byte("package main\nfunc Bar() {}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	// Mock Agent Response
	mockResponse := `
<file path="` + file1Path + `">
package main
func FooModified() {}
</file>
<file path="` + file2Path + `">
package main
func BarModified() {}
</file>
`

	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockAgentForRefactor{Response: mockResponse}, nil
	}
	defer func() { agentClientFactory = origFactory }()

	tests := []struct {
		name       string
		args       []string
		flags      map[string]string
		checkFiles bool // check if files were modified on disk
	}{
		{
			name: "Dry Run (default)",
			args: []string{file1Path, file2Path},
			flags: map[string]string{
				"prompt": "Rename functions",
			},
			checkFiles: false,
		},
		{
			name: "With Diff",
			args: []string{file1Path, file2Path},
			flags: map[string]string{
				"prompt": "Rename functions",
				"diff":   "true",
			},
			checkFiles: false,
		},
		{
			name: "In Place",
			args: []string{file1Path, file2Path},
			flags: map[string]string{
				"prompt":   "Rename functions",
				"in-place": "true",
			},
			checkFiles: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset file content for checkFiles test
			if tt.checkFiles {
				os.WriteFile(file1Path, []byte("package main\nfunc Foo() {}"), 0644)
				os.WriteFile(file2Path, []byte("package main\nfunc Bar() {}"), 0644)
			}

			cmd := &cobra.Command{Use: "recac"}
			cmd.AddCommand(refactorCmd)

			// We need to execute the specific subcommand manually or via root
			// But since refactorCmd is global in main package, we can just invoke RunE
			// However, flag parsing happens in Execute.
			// Let's manually set flags on refactorCmd

			for k, v := range tt.flags {
				refactorCmd.Flags().Set(k, v)
			}

			// Capture output
			buf := new(bytes.Buffer)
			refactorCmd.SetOut(buf)
			refactorCmd.SetErr(buf)

			err := refactorCmd.RunE(refactorCmd, tt.args)
			if err != nil {
				t.Errorf("RunE() error = %v", err)
			}

			output := buf.String()
			// t.Logf("Output: %s", output)

			// Ensure output is used or we can remove the variable if not needed for assertion directly
			_ = output

			if tt.checkFiles {
				// Verify file modification
				content1, _ := os.ReadFile(file1Path)
				if string(content1) != "package main\nfunc FooModified() {}\n" {
					t.Errorf("File1 not modified correctly. Got: %s", string(content1))
				}
				content2, _ := os.ReadFile(file2Path)
				if string(content2) != "package main\nfunc BarModified() {}\n" {
					t.Errorf("File2 not modified correctly. Got: %s", string(content2))
				}
			} else {
				// Verify NO modification
				content1, _ := os.ReadFile(file1Path)
				if string(content1) != "package main\nfunc Foo() {}" {
					t.Errorf("File1 should not be modified")
				}
			}

			// If diff is requested, check if output contains diff format
			if tt.flags["diff"] == "true" {
				if !bytes.Contains(buf.Bytes(), []byte("--- "+file1Path)) {
					t.Errorf("Output should contain diff for file1")
				}
			}
		})
	}
}
