package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockScaffoldAgent implements agent.Agent
type MockScaffoldAgent struct {
	Response string
}

func (m *MockScaffoldAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockScaffoldAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestScaffoldCommand(t *testing.T) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "scaffold-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Mock Agent Factory
	oldFactory := agentClientFactory
	defer func() { agentClientFactory = oldFactory }()

	mockResponse := `package main

import (
	"github.com/spf13/cobra"
)

var testFeatureCmd = &cobra.Command{
	Use:   "test-feature",
	Short: "A test feature",
	Run: func(cmd *cobra.Command, args []string) {
		// Do nothing
	},
}

func init() {
	rootCmd.AddCommand(testFeatureCmd)
}`
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockScaffoldAgent{Response: mockResponse}, nil
	}

	// Execute RunE directly
	cmd := &cobra.Command{}
	ctx := context.Background()
	cmd.SetContext(ctx)

	// runScaffoldCommand is defined in scaffold.go in package main
	err = runScaffoldCommand(cmd, []string{"test-feature"})
	assert.NoError(t, err)

	// Verify file created
	content, err := os.ReadFile("test-feature.go")
	assert.NoError(t, err)
	// utils.CleanCodeBlock trims space, so we expect trimmed content
	assert.Equal(t, strings.TrimSpace(mockResponse), string(content))

	// Verify test file created
	testContent, err := os.ReadFile("test-feature_test.go")
	assert.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(mockResponse), string(testContent))
}
