package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetNextVersion(t *testing.T) {
	tests := []struct {
		current  string
		commits  []string
		override string
		expected string
		wantErr  bool
	}{
		{"1.0.0", []string{"feat: new thing"}, "", "1.1.0", false},
		{"1.0.0", []string{"fix: bug"}, "", "1.0.1", false},
		{"1.0.0", []string{"chore: cleanup"}, "", "1.0.1", false}, // Defaults to patch
		{"1.0.0", []string{"feat: a", "fix: b"}, "", "1.1.0", false},
		{"1.0.0", []string{"feat: a", "BREAKING CHANGE: b"}, "", "2.0.0", false},
		{"1.0.0", []string{"feat!: a"}, "", "2.0.0", false},
		{"1.0.0", []string{"random commit"}, "", "1.0.1", false},
		{"1.0.0", []string{}, "major", "2.0.0", false},
		{"1.0.0", []string{}, "minor", "1.1.0", false},
		{"1.0.0", []string{}, "patch", "1.0.1", false},
		{"invalid", []string{"fix: a"}, "", "", true},
	}

	for _, tt := range tests {
		got, err := getNextVersion(tt.current, tt.commits, tt.override)
		if tt.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got, "Inputs: %v, %v", tt.current, tt.commits)
		}
	}
}

func TestVersionFileOps(t *testing.T) {
	tmpDir := t.TempDir()
	vFile := filepath.Join(tmpDir, "version.go")

	initialContent := `package main
var (
	version = "v1.2.3"
	commit  = "HEAD"
	date    = "2020-01-01"
)
`
	err := os.WriteFile(vFile, []byte(initialContent), 0644)
	require.NoError(t, err)

	// Test Get
	got, err := getCurrentVersion(vFile)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", got)

	// Test Update
	err = updateVersionFile(vFile, "1.3.0")
	require.NoError(t, err)

	// Verify Update
	newContent, err := os.ReadFile(vFile)
	require.NoError(t, err)
	assert.Contains(t, string(newContent), `version = "v1.3.0"`)
	// Verify date updated (not 2020-01-01)
	assert.NotContains(t, string(newContent), `date = "2020-01-01"`)
}

func TestReleaseIntegration(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()

	// Setup Git
	execGit(t, tmpDir, "init")
	execGit(t, tmpDir, "config", "user.email", "test@example.com")
	execGit(t, tmpDir, "config", "user.name", "Test User")

	// Setup files
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	err := os.Chdir(tmpDir)
	require.NoError(t, err)

	err = os.MkdirAll("cmd/recac", 0755)
	require.NoError(t, err)

	versionContent := `package main
var (
	version = "v0.1.0"
	commit  = "HEAD"
	date    = "2020-01-01"
)
`
	err = os.WriteFile("cmd/recac/version.go", []byte(versionContent), 0644)
	require.NoError(t, err)

	// Commit initial state
	execGit(t, tmpDir, "add", ".")
	execGit(t, tmpDir, "commit", "-m", "Initial commit")
	execGit(t, tmpDir, "tag", "v0.1.0")

	// Make some changes
	err = os.WriteFile("feature.txt", []byte("feat"), 0644)
	require.NoError(t, err)
	execGit(t, tmpDir, "add", "feature.txt")
	execGit(t, tmpDir, "commit", "-m", "feat: new feature")

	// Mock Agent
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &mockAgent{}, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Configure Viper
	viper.Set("provider", "mock")
	defer viper.Set("provider", "")

	// Run Release
	// We create a new command instance to ensure fresh flags
	cmd := NewReleaseCmd()
	cmd.SetArgs([]string{}) // default
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Execute
	err = cmd.Execute()
	require.NoError(t, err, "Command output: %s", buf.String())

	// Verify Version File
	content, err := os.ReadFile("cmd/recac/version.go")
	require.NoError(t, err)
	assert.Contains(t, string(content), `version = "v0.2.0"`) // feat -> minor bump

	// Verify Tag
	out := execGit(t, tmpDir, "describe", "--tags")
	assert.Contains(t, string(out), "v0.2.0")

	// Verify Changelog
	clContent, err := os.ReadFile("CHANGELOG.md")
	require.NoError(t, err)
	assert.Contains(t, string(clContent), "MOCK CHANGELOG")
	assert.Contains(t, string(clContent), "v0.2.0")
}

type mockAgent struct {}
func (m *mockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return "MOCK CHANGELOG", nil
}
func (m *mockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	return "MOCK CHANGELOG", nil
}

func execGit(t *testing.T, dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "Git %v failed: %s", args, out)
	return string(out)
}
