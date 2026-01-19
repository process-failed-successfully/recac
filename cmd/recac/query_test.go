package main

import (
	"context"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// QueryTestMockAgent allows us to mock the Agent interface for query tests
type QueryTestMockAgent struct {
	mock.Mock
}

func (m *QueryTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *QueryTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	// Simulate streaming by calling onChunk with the response
	response := args.String(0)
	if response != "" {
		onChunk(response)
	}
	return response, args.Error(1)
}

func TestQueryCmd(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-query-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Create some dummy files
	os.WriteFile("main.go", []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile("utils.go", []byte("package utils\nfunc Helper() {}"), 0644)

	// Mock Agent
	mockAgent := new(QueryTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Expectation: Agent should receive a prompt containing the question and context
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		prompt := args.String(1)
		assert.Contains(t, prompt, "Where is main?") // The question
		assert.Contains(t, prompt, "main.go")        // The context
		assert.Contains(t, prompt, "utils.go")
	}).Return("main is in main.go", nil)

	// Execute
	output, err := executeCommand(rootCmd, "query", "Where is main?")
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "main is in main.go")
}

func TestQueryCmd_Focus(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-query-test-focus")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	os.Mkdir("pkg", 0755)
	os.WriteFile("pkg/lib.go", []byte("package lib"), 0644)
	os.WriteFile("main.go", []byte("package main"), 0644)

	mockAgent := new(QueryTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		prompt := args.String(1)
		assert.Contains(t, prompt, "pkg/lib.go")
		assert.NotContains(t, prompt, "main.go") // Should be ignored due to focus
	}).Return("Analysis done", nil)

	// Note: We need to reset the queryFocus variable because it's a package-level var bound to flag
	// The executeCommand helper resets flags, but we must ensure the variable reflects that?
	// Cobra binds flags to variables. resetFlags resets the flag values.
	// We might need to manually ensure queryFocus is cleared or just rely on `resetFlags` doing its job.
	// However, `queryFocus` is a var, not just a flag value. Cobra updates the var when flag is parsed.
	// When we run `executeCommand`, it parses flags.
	// But previous tests might have left `queryFocus` dirty if `resetFlags` doesn't update the bound variable?
	// Cobra's `StringVarP` takes a pointer. `resetFlags` calls `f.Value.Set(f.DefValue)`.
	// Since `f.Value` is a `stringValue` which points to our var, setting it should update the var.
	// So `resetFlags` should work.

	_, err = executeCommand(rootCmd, "query", "--focus", "pkg", "Analyze pkg")
	assert.NoError(t, err)
}

func TestQueryCmd_Ignore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-query-test-ignore")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	os.WriteFile("secret.txt", []byte("API_KEY=123"), 0644)
	os.WriteFile("public.txt", []byte("hello"), 0644)

	mockAgent := new(QueryTestMockAgent)
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		prompt := args.String(1)
		assert.Contains(t, prompt, "public.txt")
		assert.NotContains(t, prompt, "secret.txt")
	}).Return("Safe analysis", nil)

	// We need to clear queryIgnore manually because it's a slice?
	// resetFlags handles slices by calling Set("").
	// Let's rely on that.

	_, err = executeCommand(rootCmd, "query", "--ignore", "secret.txt", "Analyze")
	assert.NoError(t, err)
}
