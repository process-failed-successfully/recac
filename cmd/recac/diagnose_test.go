package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// DiagnoseMockAgent is a unique mock for diagnose tests
type DiagnoseMockAgent struct {
	mock.Mock
}

func (m *DiagnoseMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *DiagnoseMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	if onChunk != nil {
		onChunk(args.String(0))
	}
	return args.String(0), args.Error(1)
}

// executeCommandWithInput executes a cobra command with provided stdin input
func executeCommandWithInput(root *cobra.Command, input string, args ...string) (output string, err error) {
	// Reset flags
	root.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			if strings.Contains(strings.ToLower(f.Value.Type()), "slice") {
				f.Value.Set("")
			} else {
				f.Value.Set(f.DefValue)
			}
			f.Changed = false
		}
	})

	b := new(bytes.Buffer)

	// Mock exit
	oldExit := exit
	exit = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit-%d", code))
		}
	}
	defer func() { exit = oldExit }()

	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "exit-") {
				output = b.String()
				err = nil
				return
			}
			panic(r)
		}
	}()

	root.SetArgs(args)
	root.SetOut(b)
	root.SetErr(b)
	root.SetIn(bytes.NewBufferString(input))

	err = root.Execute()
	output = b.String()
	return
}

func TestDiagnoseCmd_File(t *testing.T) {
	// Create dummy file to be referenced
	err := os.WriteFile("crash_source.go", []byte("package main\n\nfunc panicMe() { panic(1) }"), 0644)
	assert.NoError(t, err)
	defer os.Remove("crash_source.go")

	// Create log file
	logContent := "panic: 1\n\nat main.panicMe(crash_source.go:3)"
	err = os.WriteFile("crash.log", []byte(logContent), 0644)
	assert.NoError(t, err)
	defer os.Remove("crash.log")

	// Mock agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(DiagnoseMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "crash.log") || // The prompt contains the log content
			(strings.Contains(prompt, "crash_source.go") && // And the extracted content
				strings.Contains(prompt, "func panicMe()"))
	}), mock.Anything).Return("It crashed because you panicked.", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Run command
	out, err := executeCommandWithInput(rootCmd, "", "diagnose", "crash.log")
	assert.NoError(t, err)
	assert.Contains(t, out, "It crashed because you panicked")
	assert.Contains(t, out, "Scanning for referenced files")
}

func TestDiagnoseCmd_Stdin(t *testing.T) {
	// Create dummy file
	err := os.WriteFile("stdin_source.go", []byte("content"), 0644)
	assert.NoError(t, err)
	defer os.Remove("stdin_source.go")

	// Mock agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(DiagnoseMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "Error at stdin_source.go:1") &&
			strings.Contains(prompt, "content")
	}), mock.Anything).Return("Analyzed stdin.", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Run command with stdin
	input := "Error at stdin_source.go:1"

	out, err := executeCommandWithInput(rootCmd, input, "diagnose")
	assert.NoError(t, err)
	assert.Contains(t, out, "Analyzed stdin")
}
