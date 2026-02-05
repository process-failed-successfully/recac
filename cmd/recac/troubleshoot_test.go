package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"recac/internal/cmdutils"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// Helper to create a fake executable that prints to stdout/stderr and exits with code
func fakeTroubleshootExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestTroubleshootHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

// TestTroubleshootHelperProcess isn't a real test. It's used to mock exec.Command.
func TestTroubleshootHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Read command from args
	// args[0] is "sh", args[1] is "-c", args[2] is the actual command string
	args := os.Args[3:]

	if len(args) >= 3 && args[0] == "sh" && args[1] == "-c" {
		cmdStr := args[2]
		if strings.Contains(cmdStr, "fail") {
			fmt.Fprintln(os.Stdout, "some output")
			fmt.Fprintln(os.Stderr, "some error")
			os.Exit(1)
		} else if strings.Contains(cmdStr, "succeed") {
			fmt.Fprintln(os.Stdout, "success output")
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func TestRunTroubleshoot_Success(t *testing.T) {
	// Restore globals
	defer func() { execCommand = exec.Command }()

	// Mock execCommand
	execCommand = fakeTroubleshootExecCommand

	cmd := &cobra.Command{}
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))

	// "succeed" triggers exit 0 in TestHelperProcess
	err := runTroubleshoot(cmd, []string{"echo succeed"})
	assert.NoError(t, err)
}

func TestRunTroubleshoot_Fail_NoAI(t *testing.T) {
	defer func() { execCommand = exec.Command }()
	execCommand = fakeTroubleshootExecCommand

	cmd := &cobra.Command{}
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(new(bytes.Buffer))

	// Simulate "n" (No) to "Troubleshoot with AI?"
	cmd.SetIn(bytes.NewBufferString("n\n"))

	err := runTroubleshoot(cmd, []string{"echo fail"})
	assert.Error(t, err) // Should return error "command failed"
	assert.Contains(t, outBuf.String(), "Command failed (exit code 1)")
}

func TestRunTroubleshoot_Fail_AI_Fix_Apply(t *testing.T) {
	// 1. Setup Mocks
	defer func() {
		execCommand = exec.Command
		agentClientFactory = cmdutils.GetAgentClient
	}()
	oldAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &TroubleshootMockAgent{
			Response: `<file path="fixed.go">package main</file>`,
		}, nil
	}
	defer func() { agentClientFactory = oldAgentFactory }()

	defer func() { writeFileFunc = os.WriteFile }()
	filesWritten := make(map[string]string)
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		filesWritten[name] = string(data)
		return nil
	}

	execCommand = fakeTroubleshootExecCommand

	// 2. Setup Command
	cmd := &cobra.Command{}
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	cmd.SetErr(new(bytes.Buffer))

	// Input sequence:
	// 1. "y" (Troubleshoot?)
	// 2. "y" (Apply fixes?)
	// 3. "n" (Rerun?) - We stop here to verify write
	cmd.SetIn(bytes.NewBufferString("y\ny\nn\n"))

	// 3. Run
	err := runTroubleshoot(cmd, []string{"echo fail"})

	// 4. Assertions
	assert.NoError(t, err)
	assert.Contains(t, filesWritten, "fixed.go")
	assert.Equal(t, "package main\n", filesWritten["fixed.go"])
	assert.Contains(t, outBuf.String(), "Proposed Fixes")
}

// TroubleshootMockAgent implementation for testing
type TroubleshootMockAgent struct {
	Response string
}

func (m *TroubleshootMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *TroubleshootMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}
