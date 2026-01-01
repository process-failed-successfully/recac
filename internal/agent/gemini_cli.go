package agent

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/telemetry"
	"strings"
	"time"
)

// GeminiCLIClient implements the Agent interface for the Gemini CLI
type GeminiCLIClient struct {
	model   string
	workDir string
	project string
}

// NewGeminiCLIClient creates a new Gemini CLI client
// apiKey is ignored as the CLI handles auth
func NewGeminiCLIClient(apiKey, model, workDir, project string) *GeminiCLIClient {
	return &GeminiCLIClient{
		model:   model,
		workDir: workDir,
		project: project,
	}
}

// Send sends a prompt to Gemini CLI and returns the generated text
func (c *GeminiCLIClient) Send(ctx context.Context, prompt string) (string, error) {
	telemetry.TrackAgentIteration(c.project)
	agentStart := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.project, time.Since(agentStart).Seconds())
	}()

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
	// Set Cwd for the command to the project workspace
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}
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

// SendStream fallback for Gemini CLI (calls Send and emits once)
func (c *GeminiCLIClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := c.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}

// MockGeminiCLIClient is a mock for testing
type MockGeminiCLIClient struct {
	Response string
}

func (m *MockGeminiCLIClient) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}
