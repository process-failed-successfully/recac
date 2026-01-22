package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// MockAgent implementation for testing
type CatchupMockAgent struct {
	CapturedPrompt string
	Response       string
}

func (m *CatchupMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CapturedPrompt = prompt
	return m.Response, nil
}
func (m *CatchupMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}
func (m *CatchupMockAgent) Close() error { return nil }

func TestCatchupCmd(t *testing.T) {
	// 1. Setup Environment
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// 2. Setup Mocks
	mockAgent := &CatchupMockAgent{
		Response: "# Digest\n\n- Feature A added\n- Bug B fixed",
	}

	// Mock agentClientFactory
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand for git log
	// We need a helper process to mock git output
	originalExecCommand := execCommand
	execCommand = fakeCatchupExecCommand
	defer func() { execCommand = originalExecCommand }()

	// Set Env var to control the fake process
	os.Setenv("GO_TEST_CATCHUP_RESPONSE", "COMMIT::abc1234|Alice|2023-10-01|Feat: Add login|Body of commit\nCOMMIT::def5678|Bob|2023-10-02|Fix: Crash|Fixed NPE")
	defer os.Unsetenv("GO_TEST_CATCHUP_RESPONSE")

	// 3. Configure Viper/Flags
	viper.Set("provider", "mock")
	catchupSince = "24h" // Reset flag default

	// 4. Run Command
	buf := new(bytes.Buffer)
	catchupCmd.SetOut(buf)
	catchupCmd.SetArgs([]string{"--since", "48h", "--topic", "auth"})

	// Execute via ExecuteC to ensure flags are parsed?
	// Or just call runCatchup manually since flags are global variables in this package structure :(
	// Cobra flags are bound to variables. If I call runCatchup directly, I must set the variables manually.
	// But SetArgs on catchupCmd should work if I call catchupCmd.Execute()
	// However, rootCmd.Execute() is the entry point.
	// Let's call runCatchup and set variables manually to avoid cobra flag parsing complexity in unit test

	catchupSince = "48h"
	catchupTopic = "auth"
	catchupAuthor = ""
	catchupFiles = nil
	catchupOutput = ""

	err := runCatchup(catchupCmd, []string{})
	assert.NoError(t, err)

	// 5. Assertions
	output := buf.String()
	assert.Contains(t, output, "# Digest")
	assert.Contains(t, output, "- Feature A added")

	// Check prompt content
	assert.Contains(t, mockAgent.CapturedPrompt, "Summarize the following git history")
	assert.Contains(t, mockAgent.CapturedPrompt, "related to the topic 'auth'")
	assert.Contains(t, mockAgent.CapturedPrompt, "Feat: Add login")

	// Check git command arguments via the helper process output env?
	// The fake process doesn't easily report back arguments unless we capture them.
	// But we can verify behavior by the fact that we got the output.
}

func TestCatchupCmd_NoCommits(t *testing.T) {
	tempDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(origWd)

	// Setup mock agent as well
	mockAgent := &CatchupMockAgent{
		Response: "# Digest",
	}
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	originalExecCommand := execCommand
	execCommand = fakeCatchupExecCommand
	defer func() { execCommand = originalExecCommand }()

	// Ensure empty response
	os.Setenv("GO_TEST_CATCHUP_RESPONSE", "")
	defer os.Unsetenv("GO_TEST_CATCHUP_RESPONSE")

	buf := new(bytes.Buffer)
	catchupCmd.SetOut(buf)

	catchupSince = "24h"
	catchupTopic = ""

	err := runCatchup(catchupCmd, []string{})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No matching commits found")
}

// Helper process for mocking exec.Command
func fakeCatchupExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestCatchupHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)

	// Pass the response env var
	val := os.Getenv("GO_TEST_CATCHUP_RESPONSE")
	cmd.Env = []string{"GO_TEST_CATCHUP_RESPONSE=" + val}
	return cmd
}

func TestCatchupHelperProcess(t *testing.T) {
	// Identify if we are the helper process based on args
	args := os.Args
	isHelper := false
	for _, arg := range args {
		if arg == "--" {
			isHelper = true
			break
		}
	}

	if !isHelper {
		return
	}

	// Print the canned response
	// If env var is missing/empty, it prints nothing, which is what we want for empty output
	fmt.Print(os.Getenv("GO_TEST_CATCHUP_RESPONSE"))
	os.Exit(0)
}
