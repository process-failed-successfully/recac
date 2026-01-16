package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"regexp"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stripAnsi(str string) string {
	const ansi = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
	re := regexp.MustCompile(ansi)
	return re.ReplaceAllString(str, "")
}

func TestPlanCmd(t *testing.T) {
	// Setup Temp Dir
	tempDir := t.TempDir()

	// Create app_spec.txt
	err := os.WriteFile(filepath.Join(tempDir, "app_spec.txt"), []byte("Task: Build a calculator"), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockPlan := `{
		"project_name": "Calculator",
		"features": [
			{
				"id": "feat-1",
				"description": "Add functionality",
				"priority": "MVP",
				"status": "pending",
				"dependencies": { "depends_on_ids": [], "exclusive_write_paths": [], "read_only_paths": [] }
			}
		]
	}`
	mockAgent.SetResponse(mockPlan)

	// Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Override Survey
	originalSurvey := surveyAskOne
	surveyAskOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// Set confirmation to true
		if confirm, ok := response.(*bool); ok {
			*confirm = true
		}
		return nil
	}
	defer func() { surveyAskOne = originalSurvey }()

	// Execute via executeCommand helper (which uses rootCmd)
	// We must pass "plan" as the command
	output, err := executeCommand(rootCmd, "plan", "--path", tempDir)
	require.NoError(t, err)

	cleanOutput := stripAnsi(output)

	// Verify Output
	assert.Contains(t, cleanOutput, "Generating plan")
	assert.Contains(t, cleanOutput, "Calculator")
	assert.Contains(t, cleanOutput, "Add functionality")
	assert.Contains(t, cleanOutput, "Plan saved")

	// Verify File Created
	content, err := os.ReadFile(filepath.Join(tempDir, "feature_list.json"))
	require.NoError(t, err)
	assert.JSONEq(t, mockPlan, string(content))
}

func TestPlanCmd_MissingSpec(t *testing.T) {
	tempDir := t.TempDir()

	// Execute - should fail because app_spec.txt is missing
	_, err := executeCommand(rootCmd, "plan", "--path", tempDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "app_spec.txt not found")
}

func TestPlanCmd_NoSave(t *testing.T) {
	// Setup Temp Dir
	tempDir := t.TempDir()

	// Create app_spec.txt
	err := os.WriteFile(filepath.Join(tempDir, "app_spec.txt"), []byte("Task: Build a calculator"), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockPlan := `{"project_name": "Test", "features": []}`
	mockAgent.SetResponse(mockPlan)

	// Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Override Survey - Say NO
	originalSurvey := surveyAskOne
	surveyAskOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		if confirm, ok := response.(*bool); ok {
			*confirm = false
		}
		return nil
	}
	defer func() { surveyAskOne = originalSurvey }()

	output, err := executeCommand(rootCmd, "plan", "--path", tempDir)
	require.NoError(t, err)

	assert.Contains(t, output, "Plan discarded")

	// Verify File NOT Created
	_, err = os.Stat(filepath.Join(tempDir, "feature_list.json"))
	assert.True(t, os.IsNotExist(err))
}
