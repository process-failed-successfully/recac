package agent

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGeminiCLIClient_Send(t *testing.T) {
	// Save original execCommandContext and restore it after tests
	originalExecCommandContext := execCommandContext
	defer func() { execCommandContext = originalExecCommandContext }()

	// Mock execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	tests := []struct {
		name          string
		prompt        string
		model         string
		expectedResp  string
		expectedError bool
	}{
		{
			name:          "Success",
			prompt:        "Hello",
			model:         "test-model",
			expectedResp:  "Mocked OpenCode Response",
			expectedError: false,
		},
		{
			name:          "Failure",
			prompt:        "fail",
			model:         "test-model",
			expectedResp:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewGeminiCLIClient("key", tt.model, "", "test-project")

			// For Gemini, prompt is passed via Stdin.
			// Our helper process needs to read stdin to detect "fail" case if we want to test failure based on prompt.
			// But wait, TestHelperProcess checks args.
			// The mocked command args for Gemini don't contain the prompt.
			// So "fail" case won't be triggered by prompt content in args.

			// I need to update TestHelperProcess to handle this or just assume for now I test the mechanism.
			// To test failure, I might need a specific flag or just rely on `opencode_cli_test.go` updates.

			// If I can't pass "fail" in args, I should perhaps use a different model name to trigger failure?
			// GeminiCLIClient adds --model arg.
			if tt.name == "Failure" {
				// We can't easily trigger failure via args if prompt is in stdin.
				// Unless we change model to "fail-model" and update helper.
				// Let's assume I'll update helper to look for "fail" in model arg too.
				client = NewGeminiCLIClient("key", "fail-model", "", "test-project")
			}

			resp, err := client.Send(context.Background(), tt.prompt)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedResp, resp)
			}
		})
	}
}

func TestGeminiCLIClient_SendStream(t *testing.T) {
	// Save original execCommandContext and restore it after tests
	originalExecCommandContext := execCommandContext
	defer func() { execCommandContext = originalExecCommandContext }()

	// Mock execCommandContext
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.CommandContext(ctx, os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	client := NewGeminiCLIClient("key", "model", "", "test-project")
	var collectedChunks string
	resp, err := client.SendStream(context.Background(), "Hello", func(chunk string) {
		collectedChunks += chunk
	})

	assert.NoError(t, err)
	assert.Equal(t, "Mocked OpenCode Response", resp)
	assert.Equal(t, "Mocked OpenCode Response", collectedChunks)
}
