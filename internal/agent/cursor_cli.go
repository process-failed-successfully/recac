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

// CursorCLIClient implements the Agent interface for the Cursor Agent CLI
type CursorCLIClient struct {
	model string
}

// NewCursorCLIClient creates a new Cursor CLI client
// apiKey is ignored as the CLI handles auth
func NewCursorCLIClient(apiKey, model string) *CursorCLIClient {
	return &CursorCLIClient{
		model: model,
	}
}

// Send sends a prompt to Cursor CLI and returns the generated text
func (c *CursorCLIClient) Send(ctx context.Context, prompt string) (string, error) {
	// Build command: cursor-agent agent [prompt] --print --output-format text --force --workspace [cwd]
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	args := []string{
		"agent",
		prompt,
		"--print",
		"--output-format",
		"text",
		"--force",
		"--workspace",
		cwd,
	}

	if c.model != "" && c.model != "auto" {
		args = append(args, "--model", c.model)
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, "cursor-agent", args...)

	// Cursor CLI takes prompt as argument, not stdin (based on python client: cmd = ["cursor-agent", "agent", prompt, ...])
	// CAUTION: passing large prompt as arg can hit shell limits.
	// The python client does exactly this: `cmd = ["cursor-agent", "agent", prompt, ...]`
	// We will follow suit, assuming the user knows the limits or the prompt is manageable.

	// Set output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Filter environment variables similar to Python implementation if needed,
	// but for now we'll pass all and ensuring NO_OPEN_BROWSER=1
	env := os.Environ()
	env = append(env, "NO_OPEN_BROWSER=1")
	cmd.Env = env

	fmt.Printf("Running Cursor CLI: cursor-agent agent <prompt> --workspace %s ...\n", cwd)

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	if err != nil {
		stderrStr := stderr.String()

		// Check for specific errors mentioned in python client
		if strings.Contains(stderrStr, "resource_exhausted") {
			return "", fmt.Errorf("cursor agent resource exhausted: %w", err)
		}

		if stderrStr != "" {
			return "", fmt.Errorf("cursor-agent cli error (exit code %d): %s\nStderr: %s", cmd.ProcessState.ExitCode(), err, stderrStr)
		}
		return "", fmt.Errorf("cursor-agent cli error: %w", err)
	}

	output := stdout.String()
	// Log stderr if verbose
	if stderr.Len() > 0 {
		fmt.Printf("Cursor CLI Stderr: %s\n", stderr.String())
	}

	fmt.Printf("Cursor CLI completed in %v. Output length: %d\n", duration, len(output))

	return strings.TrimSpace(output), nil
}

// SendStream fallback for Cursor CLI (calls Send and emits once)
func (c *CursorCLIClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := c.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
