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
	T *testing.T
}

func (m *HealMockAgent) Send(ctx context.Context, prompt string) (string, error) {
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
	return "", fmt.Errorf("unexpected prompt content: %s", prompt)
}

func (m *HealMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestHealCmd(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}

	// Create broken file
	brokenContent := `package main

import "fmt"

func main() {
	fmt.Printl("Hello, World!")
}
`
	if err := os.WriteFile("main.go", []byte(brokenContent), 0644); err != nil {
		t.Fatalf("failed to create main.go: %v", err)
	}

	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &HealMockAgent{T: t}, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute heal
	oldCmd := healCommand
	oldRetries := healRetries
	defer func() {
		healCommand = oldCmd
		healRetries = oldRetries
	}()

	healCommand = "go run main.go"
	healRetries = 2

	// We pass empty args as they are not used by runHeal (it uses flags)
	// We need to ensure healCmd is initialized properly or just pass it as context.
	// runHeal uses healCommand/healRetries vars, so passing healCmd is mostly for printing.
	err := runHeal(healCmd, []string{})

	assert.NoError(t, err)

	// Verify file is fixed
	content, err := os.ReadFile("main.go")
	assert.NoError(t, err)
	assert.Contains(t, string(content), "fmt.Println")

	// Verify it contains no failures now
	assert.NotContains(t, string(content), "fmt.Printl(")
}
