package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type HealMockAgent struct {
	T          *testing.T
	ShouldFail bool
	NoPatches  bool
}

func (m *HealMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.ShouldFail {
		return "", fmt.Errorf("mock agent error")
	}
	if m.NoPatches {
		return "I cannot fix this.", nil
	}

	// Assert prompt contains error info
	// The error from `go run` usually contains "undefined: fmt.Printl"
	if strings.Contains(prompt, "fmt.Printl") {
		return `<file path="main.go">
package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}
</file>`, nil
	}
	// For other prompts (maybe different errors), return empty if not matching
	return "", fmt.Errorf("unexpected prompt content: %s", prompt)
}

func (m *HealMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestHealCmd(t *testing.T) {
	// Setup common variables
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	oldCmd := healCommand
	oldRetries := healRetries
	defer func() {
		healCommand = oldCmd
		healRetries = oldRetries
	}()

	tests := []struct {
		name         string
		command      string
		fileContent  string
		retries      int
		agentFail    bool
		agentNoPatch bool
		expectError  bool
		errorMsg     string
	}{
		{
			name:        "Success on first try",
			command:     "echo hello",
			retries:     1,
			expectError: false,
		},
		{
			name:    "Fix success",
			command: "go run main.go",
			fileContent: `package main
import "fmt"
func main() { fmt.Printl("Hello") }`,
			retries:     2,
			expectError: false,
		},
		{
			name:    "Failure after retries",
			command: "go run main.go",
			fileContent: `package main
import "fmt"
func main() { fmt.Printl("Hello") }`,
			retries:      0, // 0 retries means 1 attempt total (0+1)
			agentNoPatch: true,
			expectError:  true,
			errorMsg:     "failed to heal after 0 retries",
		},
		{
			name:        "No files identified",
			command:     "false", // Just fails with exit code 1, no output
			retries:     1,
			expectError: true,
			errorMsg:    "command failed but no specific files were identified",
		},
		{
			name:    "Agent failure",
			command: "go run main.go",
			fileContent: `package main
import "fmt"
func main() { fmt.Printl("Hello") }`,
			retries:     1,
			agentFail:   true,
			expectError: true,
			errorMsg:    "agent failed",
		},
		{
			name:    "Agent returns no patches",
			command: "go run main.go",
			fileContent: `package main
import "fmt"
func main() { fmt.Printl("Hello") }`,
			retries:      1,
			agentNoPatch: true,
			expectError:  true,
			errorMsg:     "failed to heal after 1 retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to change dir: %v", err)
			}

			if tt.fileContent != "" {
				if err := os.WriteFile("main.go", []byte(tt.fileContent), 0644); err != nil {
					t.Fatalf("failed to create main.go: %v", err)
				}
			}

			agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
				return &HealMockAgent{T: t, ShouldFail: tt.agentFail, NoPatches: tt.agentNoPatch}, nil
			}

			healCommand = tt.command
			healRetries = tt.retries

			err := runHeal(healCmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
