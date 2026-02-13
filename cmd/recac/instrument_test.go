package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"recac/internal/agent"
)

func TestInstrumentCmd(t *testing.T) {
	// 1. Setup Mock Agent
	mockAgent := agent.NewMockAgent()
	expectedInstrumentation := `package main

import "fmt"

func main() {
    // Instrumented
	fmt.Println("Hello")
}`
	mockAgent.SetResponse(expectedInstrumentation)

	// 2. Mock Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 3. Create Temp File
	tmpFile, err := os.CreateTemp("", "test_instrument_*.go")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	originalContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}`
	if _, err := tmpFile.WriteString(originalContent); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// 4. Run Command (Stdout)
	cmd := NewInstrumentCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{}) // ignore stderr

	// Reset flags (cobra flags are persistent on the command struct instance)
	// We are creating a new command instance via NewInstrumentCmd so flags should be fresh?
	// Wait, instrumentCmd is a global variable in instrument.go but NewInstrumentCmd creates a NEW struct.
	// Ah, in instrument.go:
	/*
		func NewInstrumentCmd() *cobra.Command {
			cmd := &cobra.Command{...}
			// ...
			cmd.Flags().StringVar(&instrumentType, ...)
			return cmd
		}
		var instrumentCmd = NewInstrumentCmd()
	*/
	// The flags bind to GLOBAL variables `instrumentType`, `instrumentInPlace`, etc.
	// This is bad for testing parallel tests, but fine for sequential.
	// We must reset the global variables.
	instrumentType = "otel"
	instrumentInPlace = false
	instrumentDiff = false

	args := []string{tmpFile.Name()}
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command execution failed: %v", err)
	}

	output := outBuf.String()
	if strings.TrimSpace(output) != strings.TrimSpace(expectedInstrumentation) {
		t.Errorf("Expected output:\n%s\nGot:\n%s", expectedInstrumentation, output)
	}

	// 5. Run Command (In-Place)
	cmdInPlace := NewInstrumentCmd()
	cmdInPlace.SetOut(&bytes.Buffer{})
	cmdInPlace.SetErr(&bytes.Buffer{})
	// Must pass the flag explicitly because cobra resets bound variables to default if flag is missing
	argsInPlace := append([]string{"--in-place"}, args...)
	cmdInPlace.SetArgs(argsInPlace)

	if err := cmdInPlace.Execute(); err != nil {
		t.Fatalf("In-place command failed: %v", err)
	}

	// Verify file content
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(content)) != strings.TrimSpace(expectedInstrumentation) {
		t.Errorf("File content mismatch. Expected:\n%s\nGot:\n%s", expectedInstrumentation, string(content))
	}
}

func TestInstrumentCmd_Stdin(t *testing.T) {
	// Mock Agent
	mockAgent := agent.NewMockAgent()
	expected := "instrumented code"
	mockAgent.SetResponse(expected)

	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Input
	input := "original code"

	// Command
	cmd := NewInstrumentCmd()
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetIn(strings.NewReader(input))

	// Reset globals
	instrumentType = "otel"
	instrumentInPlace = false

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if outBuf.String() != expected {
		t.Errorf("Expected %q, got %q", expected, outBuf.String())
	}
}
