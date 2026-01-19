package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

type MockSpecAgent struct {
	mock.Mock
}

func (m *MockSpecAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockSpecAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	// Simulate streaming
	response := args.String(0)
	if onChunk != nil {
		onChunk(response)
	}
	return response, args.Error(1)
}

func TestSpecCmd(t *testing.T) {
	// Setup temporary directory
	tempDir, err := os.MkdirTemp("", "recac-spec-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create some files
	err = os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main"), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "README.md"), []byte("# My Project"), 0644)
	assert.NoError(t, err)

	// Change CWD to tempDir
	origCwd, _ := os.Getwd()
	defer os.Chdir(origCwd)
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// Mock the agent factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := new(MockSpecAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	expectedSpec := "Project: My Project\nFeature: Main"

	t.Run("Generate spec", func(t *testing.T) {
		t.Log("Starting Generate spec test")
		mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return(expectedSpec, nil).Once()

		cmd := &cobra.Command{RunE: runSpec}
		cmd.Flags().StringVarP(&specOutput, "output", "o", "app_spec.txt", "Output file path")
		cmd.Flags().StringSliceVarP(&specExclude, "exclude", "e", []string{}, "Glob patterns to exclude")

		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		// Run
		cmd.SetArgs([]string{"--output", "app_spec.txt"})
		err := cmd.Execute()
		assert.NoError(t, err)

		// Check output file
		content, err := os.ReadFile("app_spec.txt")
		assert.NoError(t, err)
		assert.Equal(t, expectedSpec, string(content))
	})

	t.Run("Generate spec with custom output", func(t *testing.T) {
		mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return(expectedSpec, nil).Once()

		outputFile := "custom_spec.txt"

		cmd := &cobra.Command{RunE: runSpec}
		cmd.Flags().StringVarP(&specOutput, "output", "o", "app_spec.txt", "Output file path")
		cmd.Flags().StringSliceVarP(&specExclude, "exclude", "e", []string{}, "Glob patterns to exclude")

		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		cmd.SetArgs([]string{"--output", outputFile})

		err := cmd.Execute()
		assert.NoError(t, err)

		// Check output file
		content, err := os.ReadFile(outputFile)
		assert.NoError(t, err)
		assert.Equal(t, expectedSpec, string(content))
	})

	t.Run("Generate spec with exclude", func(t *testing.T) {
		mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).Return(expectedSpec, nil).Once()

		// Add a file that should be excluded
		err := os.WriteFile(filepath.Join(tempDir, "secret.key"), []byte("secret"), 0644)
		assert.NoError(t, err)

		cmd := &cobra.Command{RunE: runSpec}
		cmd.Flags().StringVarP(&specOutput, "output", "o", "app_spec.txt", "Output file path")
		cmd.Flags().StringSliceVarP(&specExclude, "exclude", "e", []string{}, "Glob patterns to exclude")

		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		cmd.SetArgs([]string{"--output", "spec_ex.txt", "--exclude", "*.key"})

		err = cmd.Execute()
		assert.NoError(t, err)
	})
}
