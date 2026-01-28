package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"
	"recac/internal/db"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanCmd(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy app_spec.txt
	specPath := filepath.Join(tmpDir, "app_spec.txt")
	specContent := "Task: Build a calculator app."
	err = os.WriteFile(specPath, []byte(specContent), 0644)
	require.NoError(t, err)

	// Change working directory to temp dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Mock Agent Response
	mockResponse := "```json\n" +
		`{
  "project_name": "Calculator",
  "features": [
    {
      "id": "feat-1",
      "category": "functional",
      "description": "Add two numbers",
      "status": "pending",
      "steps": ["Step 1"],
      "dependencies": {
        "depends_on_ids": []
      }
    }
  ]
}` + "\n```"
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse(mockResponse)

	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Execute Plan Command
	cmd := NewPlanCmd()

	// Create a fresh root command for testing to avoid global state issues
	testRoot := &cobra.Command{Use: "test"}
	testRoot.AddCommand(cmd)

	// Redirect output
	testRoot.SetOut(os.Stdout)
	testRoot.SetErr(os.Stderr)

	// Set args including the subcommand name
	testRoot.SetArgs([]string{"plan", specPath})

	err = testRoot.Execute()
	require.NoError(t, err)

	// Verify Output File
	outputPath := filepath.Join(tmpDir, "feature_list.json")
	require.FileExists(t, outputPath)

	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var list db.FeatureList
	err = json.Unmarshal(content, &list)
	require.NoError(t, err)

	assert.Equal(t, "Calculator", list.ProjectName)
	assert.Len(t, list.Features, 1)
	assert.Equal(t, "feat-1", list.Features[0].ID)
}

func TestPlanCmd_MissingSpec(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-test-missing")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Override factory to avoid actual calls
	mockAgent := agent.NewMockAgent()
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	cmd := NewPlanCmd()

	testRoot := &cobra.Command{Use: "test"}
	testRoot.AddCommand(cmd)
	testRoot.SetArgs([]string{"plan", "non_existent_spec.txt"})
	testRoot.SetOut(os.Stdout)
	testRoot.SetErr(os.Stderr)

	err = testRoot.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read spec file")
}

func TestPlanCmd_InvalidJSON(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-test-invalid")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("task"), 0644)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("This is not JSON")

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	cmd := NewPlanCmd()

	testRoot := &cobra.Command{Use: "test"}
	testRoot.AddCommand(cmd)
	testRoot.SetArgs([]string{"plan", specPath})
	testRoot.SetOut(os.Stdout)
	testRoot.SetErr(os.Stderr)

	err = testRoot.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse agent response as JSON")
}

func TestPlanCmd_TextBeforeJSON(t *testing.T) {
	// Setup temporary workspace
	tmpDir, err := os.MkdirTemp("", "recac-plan-test-text")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	specPath := filepath.Join(tmpDir, "app_spec.txt")
	os.WriteFile(specPath, []byte("Task: Do things"), 0644)

	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(tmpDir)

	// Response with text before and after the JSON block
	mockResponse := `
Here is your plan:
` + "```json" + `
{
  "project_name": "Text Project",
  "features": []
}
` + "```" + `
Hope this helps!
`
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse(mockResponse)

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	cmd := NewPlanCmd()

	testRoot := &cobra.Command{Use: "test"}
	testRoot.AddCommand(cmd)
	testRoot.SetArgs([]string{"plan", specPath})
	testRoot.SetOut(os.Stdout)
	testRoot.SetErr(os.Stderr)

	err = testRoot.Execute()
	require.NoError(t, err)

	outputPath := filepath.Join(tmpDir, "feature_list.json")
	require.FileExists(t, outputPath)

	content, _ := os.ReadFile(outputPath)
	var list db.FeatureList
	err = json.Unmarshal(content, &list)
	require.NoError(t, err)
	assert.Equal(t, "Text Project", list.ProjectName)
}
