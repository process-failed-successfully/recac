package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestTranslateCmd(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "recac-translate-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create dummy input file
	inputFile := filepath.Join(tmpDir, "script.py")
	inputContent := "print('Hello World')"
	err = os.WriteFile(inputFile, []byte(inputContent), 0644)
	assert.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	expectedTranslation := `package main
import "fmt"
func main() {
	fmt.Println("Hello World")
}`
	mockAgent.SetResponse("```go\n" + expectedTranslation + "\n```")

	// Mock Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Test Case 1: Normal Translation (Python -> Go)
	outputFile := filepath.Join(tmpDir, "script.go")
	// cobra commands are structs. We can just invoke runTranslate if we export it or use the var.
	// But `runTranslate` is unexported `runTranslate`. I can access it because I am in `main` package.
	// Wait, test files in `cmd/recac` usually have `package main`.

	// Let's verify package name of other tests in cmd/recac
	// e.g. cmd/recac/start_test.go
	// It is `package main`.

	// So I can call `runTranslate`.

	// Setup Flags
	// Since `runTranslate` accesses flags via `translateTarget` var, I need to set them.
	// But `translateTarget` is a package var.
	// This is NOT thread safe. I should assume tests are not parallel or use a mutex.
	// Usually `cobra` flags are bound to vars.
	// `runTranslate` reads `translateTarget` global var directly in my implementation?
	// Let's check `translate.go`.
	/*
		func runTranslate(cmd *cobra.Command, args []string) error {
			inputFile := args[0]
			...
			outputPath := translateOutput // Global var
			if outputPath == "" {
				ext := getExtensionForLanguage(translateTarget) // Global var
			...
	*/
	// Yes, they use global vars. This is common in simple Cobra apps but bad for testing.
	// I should reset them after test.

	// Reset flags
	resetFlags := func() {
		translateTarget = ""
		translateOutput = ""
		translateForce = false
	}
	defer resetFlags()

	// 1. Success Case
	translateTarget = "go"
	translateOutput = outputFile

	// Create a dummy command to pass (it writes to stdout)
	dummyCmd := &cobra.Command{}

	err = runTranslate(dummyCmd, []string{inputFile})
	assert.NoError(t, err)

	// Verify output
	content, err := os.ReadFile(outputFile)
	assert.NoError(t, err)
	assert.Equal(t, expectedTranslation, string(content))

	// 2. Error Case: Output exists without force
	err = runTranslate(dummyCmd, []string{inputFile})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// 3. Success Case: Force overwrite
	translateForce = true
	err = runTranslate(dummyCmd, []string{inputFile})
	assert.NoError(t, err)

	// 4. Success Case: Infer output filename
	translateOutput = "" // Should infer script.go (but it exists, so with force it should work)
	translateTarget = "go"
	translateForce = true

	err = runTranslate(dummyCmd, []string{inputFile})
	assert.NoError(t, err)

	// Check if inferred path was correct (script.go in same dir)
	// We already verified contents.

	// 5. Infer output filename for different language
	translateTarget = "rust"
	translateOutput = ""
	translateForce = false
	expectedRustFile := filepath.Join(tmpDir, "script.rs")

	// Update mock for Rust
	mockAgent.SetResponse("fn main() {}")

	err = runTranslate(dummyCmd, []string{inputFile})
	assert.NoError(t, err)

	if _, err := os.Stat(expectedRustFile); os.IsNotExist(err) {
		t.Errorf("Expected inferred output file %s to exist", expectedRustFile)
	}
}
