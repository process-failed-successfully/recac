package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DirectiveAgent wraps an existing Agent and prepends a system directive to all prompts.
// The directive is read from the .recac/directive file in the workDir.
type DirectiveAgent struct {
	agent   Agent
	workDir string
}

// NewDirectiveAgent creates a new DirectiveAgent.
func NewDirectiveAgent(agent Agent, workDir string) *DirectiveAgent {
	return &DirectiveAgent{
		agent:   agent,
		workDir: workDir,
	}
}

// Send implements the Agent interface.
func (d *DirectiveAgent) Send(ctx context.Context, prompt string) (string, error) {
	fullPrompt, err := d.prependDirective(prompt)
	if err != nil {
		// Log error but continue with original prompt to avoid breaking the flow
		// In a real logger we would log this. For now, we proceed.
		return d.agent.Send(ctx, prompt)
	}
	return d.agent.Send(ctx, fullPrompt)
}

// SendStream implements the Agent interface.
func (d *DirectiveAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	fullPrompt, err := d.prependDirective(prompt)
	if err != nil {
		return d.agent.SendStream(ctx, prompt, onChunk)
	}
	return d.agent.SendStream(ctx, fullPrompt, onChunk)
}

// prependDirective reads the directive file and prepends it to the prompt.
func (d *DirectiveAgent) prependDirective(prompt string) (string, error) {
	directivePath := filepath.Join(d.workDir, ".recac", "directive")

	// Check if file exists
	if _, err := os.Stat(directivePath); os.IsNotExist(err) {
		return prompt, nil
	}

	content, err := os.ReadFile(directivePath)
	if err != nil {
		return prompt, fmt.Errorf("failed to read directive file: %w", err)
	}

	directive := strings.TrimSpace(string(content))
	if directive == "" {
		return prompt, nil
	}

	// Format the directive clearly
	// We use a format that most LLMs will interpret as a system instruction
	return fmt.Sprintf("[PROJECT DIRECTIVE]: %s\n---\n%s", directive, prompt), nil
}
