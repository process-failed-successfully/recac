package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHelperProcess isn't a real test. It's used to mock exec.Command
// This is a standard pattern for testing os/exec in Go.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// Check arguments to decide what to print
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command provided\n")
		os.Exit(2)
	}

	cmd, args := args[0], args[1:]

	// Check for failure triggers in args
	shouldFail := false
	for _, arg := range args {
		if arg == "fail" || strings.Contains(arg, "fail-model") {
			shouldFail = true
			break
		}
	}

	if shouldFail {
		fmt.Fprint(os.Stderr, "Mock error")
		os.Exit(1)
	}

	switch cmd {
	case "opencode", "cursor-agent", "gemini":
		fmt.Fprint(os.Stdout, "Mocked OpenCode Response")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n", cmd)
		os.Exit(2)
	}
}

func TestOpenCodeCLIClient_Send(t *testing.T) {
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
			client := NewOpenCodeCLIClient("key", tt.model, "", "test-project")
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

func TestOpenCodeCLIClient_SendStream(t *testing.T) {
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

	client := NewOpenCodeCLIClient("key", "model", "", "test-project")
	var collectedChunks string
	resp, err := client.SendStream(context.Background(), "Hello", func(chunk string) {
		collectedChunks += chunk
	})

	assert.NoError(t, err)
	assert.Equal(t, "Mocked OpenCode Response", resp)
	assert.Equal(t, "Mocked OpenCode Response", collectedChunks)
}
