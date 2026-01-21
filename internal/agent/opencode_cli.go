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

var execCommandContext = exec.CommandContext

// OpenCodeCLIClient implements the Agent interface for the OpenCode CLI
type OpenCodeCLIClient struct {
	model   string
	workDir string
	project string
}

// NewOpenCodeCLIClient creates a new OpenCode CLI client
func NewOpenCodeCLIClient(apiKey, model, workDir, project string) *OpenCodeCLIClient {
	return &OpenCodeCLIClient{
		model:   model,
		workDir: workDir,
		project: project,
	}
}

// Send sends a prompt to OpenCode CLI and returns the generated text
func (c *OpenCodeCLIClient) Send(ctx context.Context, prompt string) (string, error) {
	telemetry.TrackAgentIteration(c.project)
	agentStart := time.Now()
	defer func() {
		telemetry.ObserveAgentLatency(c.project, time.Since(agentStart).Seconds())
	}()

	// Build command: opencode run
	args := []string{"run"}

	if c.model != "" && c.model != "auto" {
		args = append(args, "--model", c.model)
	}

	// Add the prompt as the last argument
	args = append(args, prompt)

	// Prepare command
	cmd := execCommandContext(ctx, "opencode", args...)

	// Set output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if c.workDir != "" {
		cmd.Dir = c.workDir
	}

	// Preserve existing env (mocks) and append system env
	env := cmd.Env
	if len(env) == 0 {
		env = os.Environ()
	} else {
		env = append(env, os.Environ()...)
	}
	cmd.Env = env

	// Mask prompt in logs if too long
	logArgs := args
	if len(prompt) > 50 {
		logArgs = make([]string, len(args))
		copy(logArgs, args)
		logArgs[len(logArgs)-1] = prompt[:20] + "..." + prompt[len(prompt)-20:]
	}
	fmt.Printf("Running OpenCode CLI: opencode %s\n", strings.Join(logArgs, " "))

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	if err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", fmt.Errorf("opencode cli error (exit code %d): %s\nStderr: %s", cmd.ProcessState.ExitCode(), err, stderrStr)
		}
		return "", fmt.Errorf("opencode cli error: %w", err)
	}

	output := stdout.String()
	if stderr.Len() > 0 {
		fmt.Printf("OpenCode CLI Stderr: %s\n", stderr.String())
	}

	fmt.Printf("OpenCode CLI completed in %v. Output length: %d\n", duration, len(output))
	return strings.TrimSpace(output), nil
}

// SendStream fallback for OpenCode CLI (calls Send and emits once)
func (c *OpenCodeCLIClient) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	resp, err := c.Send(ctx, prompt)
	if err == nil && onChunk != nil {
		onChunk(resp)
	}
	return resp, err
}
