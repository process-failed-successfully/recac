package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnounceCmd_Changelog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CHANGELOG.md
	err := os.WriteFile(filepath.Join(tmpDir, "CHANGELOG.md"), []byte("## v1.0.0\n- Feature A\n- Fix B"), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("🚀 v1.0.0 is out! \nFeatures:\n- Feature A\n#release")

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Execute command in tmpDir
	// We need to change CWD because runAnnounce uses os.Getwd()
	wd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(wd)

	output, err := executeCommand(rootCmd, "announce", "--platform", "twitter")
	require.NoError(t, err)
	assert.Contains(t, output, "🚀 v1.0.0 is out!")
}

func TestAnnounceCmd_GitLog(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Exciting update! We just shipped Feature X.")

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock Git Client
	mockGit := &MockGitClient{
		RepoExistsFunc: func(path string) bool { return true },
		LatestTagFunc: func(path string) (string, error) { return "v0.9.0", nil },
		LogFunc: func(path string, args ...string) ([]string, error) {
			return []string{"feat: Feature X", "fix: Bug Y"}, nil
		},
	}
	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient { return mockGit }
	defer func() { gitClientFactory = originalGitFactory }()

	// Execute command in tmpDir
	wd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(wd)

	output, err := executeCommand(rootCmd, "announce", "--tone", "funny")
	require.NoError(t, err)
	assert.Contains(t, output, "Exciting update!")
}

func TestAnnounceCmd_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create CHANGELOG.md
	os.WriteFile(filepath.Join(tmpDir, "CHANGELOG.md"), []byte("Changes"), 0644)

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockAgent.SetResponse("Draft content")

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	wd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(wd)

	outFile := "draft.txt"
	output, err := executeCommand(rootCmd, "announce", "--output", outFile)
	require.NoError(t, err)
	assert.Contains(t, output, "Announcement written to draft.txt")

	content, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Equal(t, "Draft content", string(content))
}
