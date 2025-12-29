package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// GeminiCLIClient implements the Agent interface for the Gemini CLI
type GeminiCLIClient struct {
	model string
}

// NewGeminiCLIClient creates a new Gemini CLI client
// apiKey is ignored as the CLI handles auth
func NewGeminiCLIClient(apiKey, model string) *GeminiCLIClient {
	return &GeminiCLIClient{
		model: model,
	}
}

// Send sends a prompt to Gemini CLI and returns the generated text
func (c *GeminiCLIClient) Send(ctx context.Context, prompt string) (string, error) {
	// Build command: gemini --output-format text --approval-mode yolo
	args := []string{"--output-format", "text", "--approval-mode", "yolo"}

	if c.model != "" && c.model != "auto" {
		args = append(args, "--model", c.model)
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, "gemini", args...)

	// Set input
	cmd.Stdin = strings.NewReader(prompt)

	// Set output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	// Note: We don't set a specific Cwd here as the CLI should run from where recac runs,
	// or it might rely on the project directory. The python equivalent passes the project dir.
	// For now, let's inherit current working directory
	cmd.Env = os.Environ()
	// Ensure stdout is captured in text mode
	// Python side does: cmd = ["gemini", "--output-format", "text", "--approval-mode", "yolo"]

	fmt.Printf("Running Gemini CLI: gemini %s\n", strings.Join(args, " "))

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("gemini cli error (exit code %d): %s\nStderr: %s", cmd.ProcessState.ExitCode(), err, stderrStr)
		}
		return "", fmt.Errorf("gemini cli error: %w", err)
	}

	output := stdout.String()
	// Log stderr if verbose (optional, simplistic for now)
	if stderr.Len() > 0 {
		fmt.Printf("Gemini CLI Stderr: %s\n", stderr.String())
	}

	fmt.Printf("Gemini CLI completed in %v. Output length: %d\n", duration, len(output))

	// In text output mode, the stdout is the response content
	return strings.TrimSpace(output), nil
}

// MockGeminiCLIClient is a mock for testing
type MockGeminiCLIClient struct {
	Response string
}

func (m *MockGeminiCLIClient) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}
