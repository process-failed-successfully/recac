package runner

import (
	"context"
	"fmt"
	"log/slog"
	"recac/internal/notify"
	"testing"
)

func TestSession_ProcessResponse_MockAgentDefault(t *testing.T) {
	mockDocker := &MockDockerForExec{}
	s := &Session{
		Docker:      mockDocker,
		ContainerID: "test-container",
		Notifier:    notify.NewManager(func(string, ...interface{}) {}),
		Logger:      slog.Default(),
	}

	// Construct the exact response string from MockAgent
	prompt := "Some prompt text that might be long..."
    responsePrefix := "Mock agent response"

    // From internal/agent/mock.go
    // Note: I am copying the string format exactly as I read it from internal/agent/mock.go
    // response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op\n```",
	//	m.responsePrefix, len(prompt), truncateString(prompt, 100))

    response := fmt.Sprintf("%s:\n\nI received your prompt (%d characters). In mock mode, I would process this request and provide a response. The actual implementation would call the AI provider API here.\n\nPrompt preview: %s...\n\n```bash\n# no-op\n```",
		responsePrefix, len(prompt), prompt)

    out, err := s.ProcessResponse(context.Background(), response)
    if err != nil {
        t.Fatalf("ProcessResponse failed: %v", err)
    }

    // Check if executed
    found := false
    for _, executed := range mockDocker.ExecutedCmds {
        // strings.Join(cmd, " ") -> "/bin/bash -c # no-op"
        // strings.Contains is safer
        if executed == "/bin/bash -c # no-op" {
             found = true
             break
        }
    }

    if !found {
        t.Logf("Output: %s", out)
        t.Logf("Executed commands: %v", mockDocker.ExecutedCmds)
        t.Errorf("Expected execution of no-op block, got none. Response: %q", response)
    }
}
