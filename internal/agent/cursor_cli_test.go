package agent

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCursorCLIClient_Send(t *testing.T) {
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
			expectedResp:  "Mocked OpenCode Response", // TestHelperProcess returns this for default
			expectedError: false,
		},
		{
			name:          "Failure",
			prompt:        "fail", // Triggers exit 1 in TestHelperProcess if implemented generally
			model:         "test-model",
			expectedResp:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCursorCLIClient("key", tt.model, "test-project")
			// We need to pass "fail" as last arg to trigger failure in helper if structured that way.
			// But CursorCLI passes prompt as argument.

			// Note: The shared TestHelperProcess in opencode_cli_test.go is inside package agent.
			// Since this test file is also in package agent, we can reuse it IF it was exported or in the same package.
			// It is not exported (starts with Test...), but test code in same package can see it.
			// However, TestHelperProcess checks for command "opencode".
			// We need it to handle "cursor-agent" too.

			// I need to update TestHelperProcess in opencode_cli_test.go to handle other commands.
			// Or I can define a new helper here. But functions must be unique per package.
			// TestHelperProcess is already defined in opencode_cli_test.go.

			// Strategy: Update opencode_cli_test.go to be more generic or just add cases there.
			// Since I cannot edit opencode_cli_test.go in this step easily without context switching,
			// I will rename the helper here if I define one, but `execCommand` calls `TestHelperProcess`.
			// So `TestHelperProcess` MUST be the entry point.

			// I will assume I need to update `opencode_cli_test.go` to include cases for cursor and gemini.

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

func TestCursorCLIClient_SendStream(t *testing.T) {
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

	client := NewCursorCLIClient("key", "model", "test-project")
	var collectedChunks string
	resp, err := client.SendStream(context.Background(), "Hello", func(chunk string) {
		collectedChunks += chunk
	})

	assert.NoError(t, err)
	assert.Equal(t, "Mocked OpenCode Response", resp) // Shared mock response
	assert.Equal(t, "Mocked OpenCode Response", collectedChunks)
}
