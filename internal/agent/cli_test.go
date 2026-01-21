package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAgentHelperProcess isn't a real test. It's used to mock exec.Command
// because we cannot mock os/exec directly.
func TestAgentHelperProcess(t *testing.T) {
	// Parse command to determine behavior
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}

	// If no "--" found, we are running as a normal test (not helper)
	if len(args) == 0 {
		return
	}

	cmd := args[0]
	switch cmd {
	case "cursor-agent":
		handleCursorAgent(args)
	case "opencode":
		handleOpenCode(args)
	case "gemini":
		handleGemini(args)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		os.Exit(1)
	}
	os.Exit(0)
}

func handleCursorAgent(args []string) {
	// args[0] is "cursor-agent"
	// check args for failure case
	if len(args) > 1 && args[1] == "agent" && args[2] == "fail" {
		fmt.Fprintf(os.Stderr, "cursor agent failed\n")
		os.Exit(1)
	}
	fmt.Print("Cursor Agent Output")
}

func handleOpenCode(args []string) {
	// opencode run --model x prompt
	// check for failure
	for _, arg := range args {
		if arg == "fail" {
			fmt.Fprintf(os.Stderr, "opencode failed\n")
			os.Exit(1)
		}
	}
	fmt.Print("OpenCode Output")
}

func handleGemini(args []string) {
	// gemini --output-format text ...
	// check stdin for prompt? difficult in helper without reading stdin
	// We can check args.
	for _, arg := range args {
		if arg == "fail" {
			fmt.Fprintf(os.Stderr, "gemini failed\n")
			os.Exit(1)
		}
	}
	fmt.Print("Gemini Output")
}

func fakeExecCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestAgentHelperProcess", "--", name}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	return cmd
}

func TestCursorCLIClient_Send(t *testing.T) {
	// Save old execCommandContext and restore after test
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	client := NewCursorCLIClient("", "auto", "test-project")

	t.Run("Success", func(t *testing.T) {
		resp, err := client.Send(context.Background(), "hello")
		assert.NoError(t, err)
		assert.Contains(t, resp, "Cursor Agent Output")
	})

	t.Run("Failure", func(t *testing.T) {
		resp, err := client.Send(context.Background(), "fail")
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "cursor agent failed")
		}
		assert.Equal(t, "", resp)
	})

	t.Run("SendStream", func(t *testing.T) {
		chunks := 0
		resp, err := client.SendStream(context.Background(), "hello", func(chunk string) {
			chunks++
			assert.Contains(t, chunk, "Cursor Agent Output")
		})
		assert.NoError(t, err)
		assert.Contains(t, resp, "Cursor Agent Output")
		assert.Equal(t, 1, chunks)
	})
}

func TestOpenCodeCLIClient_Send(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	client := NewOpenCodeCLIClient("", "auto", "", "test-project")

	t.Run("Success", func(t *testing.T) {
		resp, err := client.Send(context.Background(), "hello")
		assert.NoError(t, err)
		assert.Contains(t, resp, "OpenCode Output")
	})

	t.Run("Failure", func(t *testing.T) {
		resp, err := client.Send(context.Background(), "fail")
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "opencode failed")
		}
		assert.Equal(t, "", resp)
	})

	t.Run("SendStream", func(t *testing.T) {
		chunks := 0
		resp, err := client.SendStream(context.Background(), "hello", func(chunk string) {
			chunks++
			assert.Contains(t, chunk, "OpenCode Output")
		})
		assert.NoError(t, err)
		assert.Contains(t, resp, "OpenCode Output")
		assert.Equal(t, 1, chunks)
	})

}

func TestGeminiCLIClient_Send(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	client := NewGeminiCLIClient("", "auto", "", "test-project")

	t.Run("Success", func(t *testing.T) {
		resp, err := client.Send(context.Background(), "hello")
		assert.NoError(t, err)
		assert.Contains(t, resp, "Gemini Output")
	})

    clientFail := NewGeminiCLIClient("", "fail", "", "test-project")
	t.Run("Failure", func(t *testing.T) {
		resp, err := clientFail.Send(context.Background(), "hello")
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "gemini failed")
		}
		assert.Equal(t, "", resp)
	})

	t.Run("SendStream", func(t *testing.T) {
		chunks := 0
		resp, err := client.SendStream(context.Background(), "hello", func(chunk string) {
			chunks++
			assert.Contains(t, chunk, "Gemini Output")
		})
		assert.NoError(t, err)
		assert.Contains(t, resp, "Gemini Output")
		assert.Equal(t, 1, chunks)
	})
}
