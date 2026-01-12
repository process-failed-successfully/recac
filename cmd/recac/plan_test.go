package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPlannerAgent is a mock implementation of the agent.Agent interface for testing.
type mockPlannerAgent struct {
	// PlanToReturn is the JSON string the agent will return when Send is called.
	PlanToReturn string
	// CapturedPrompt stores the prompt that was sent to the agent.
	CapturedPrompt string
	// ReturnError is an error to be returned by the Send method, for testing failure cases.
	ReturnError error
}

func (m *mockPlannerAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.CapturedPrompt = prompt
	if m.ReturnError != nil {
		return "", m.ReturnError
	}
	// The planner expects a JSON block, potentially inside markdown.
	return "```json\n" + m.PlanToReturn + "\n```", nil
}

func (m *mockPlannerAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	// Not used by the plan command, but required by the interface.
	return m.Send(ctx, prompt)
}

func TestPlanCmd(t *testing.T) {
	// Setup: Define the plan our mock agent will return.
	expectedPlan := db.FeatureList{
		Features: []db.Feature{
			{ID: "main.go", Description: "Create the main file"},
			{ID: "server.go", Description: "Setup the server"},
		},
	}
	planBytes, err := json.Marshal(expectedPlan)
	require.NoError(t, err)
	mockPlanJSON := string(planBytes)

	// Setup: Create the mock agent and override the factory.
	mockAgent := &mockPlannerAgent{PlanToReturn: mockPlanJSON}
	originalGetAgentClient := getAgentClient
	getAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	// Restore the original factory after the test.
	defer func() { getAgentClient = originalGetAgentClient }()

	t.Run("happy path - generate plan", func(t *testing.T) {
		// Setup: Capture output directly to a buffer to bypass issues with executeCommand.
		var out bytes.Buffer
		rootCmd, _, _ := newRootCmd()
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out) // Capture stderr as well for debugging

		// Execute the command
		args := []string{"plan", "Create a new web server"}
		rootCmd.SetArgs(args)
		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify the output
		output := out.String()
		var resultPlan db.FeatureList
		// The command prints informational messages to stderr, which we are capturing.
		// We need to find the start of the JSON block.
		jsonStartIndex := strings.Index(output, "{")
		require.True(t, jsonStartIndex >= 0, "Could not find start of JSON in output")
		jsonOutput := output[jsonStartIndex:]

		err = json.Unmarshal([]byte(jsonOutput), &resultPlan)
		require.NoError(t, err, "Output should be valid JSON. Full output:\n%s", output)
		assert.Equal(t, expectedPlan, resultPlan, "The output plan should match the expected plan")

		// Verify the prompt sent to the agent
		assert.Contains(t, mockAgent.CapturedPrompt, "Create a new web server")
	})

	t.Run("error - no prompt provided", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		_, err := executeCommand(rootCmd, "plan")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires at least 1 arg(s), only received 0")
	})

	t.Run("with app_spec context", func(t *testing.T) {
		// Setup: Create a temporary directory and an app_spec.txt file.
		tempDir := t.TempDir()
		specPath := filepath.Join(tempDir, "app_spec.txt")
		specContent := "This is the existing project specification."
		err := os.WriteFile(specPath, []byte(specContent), 0644)
		require.NoError(t, err)

		// Execute the command
		rootCmd, _, _ := newRootCmd()
		// We can use executeCommand here because we are only checking the captured prompt,
		// not the command's output.
		_, err = executeCommand(rootCmd, "plan", "--path", tempDir, "Add a new feature.")
		require.NoError(t, err)

		// Verify that the prompt sent to the agent contains both the spec and the new request.
		expectedPromptFragment := "Existing context:\nThis is the existing project specification.\n\nNew request:\nAdd a new feature."
		assert.True(t, strings.Contains(mockAgent.CapturedPrompt, expectedPromptFragment), "Prompt should contain context from app_spec.txt")
	})
}
