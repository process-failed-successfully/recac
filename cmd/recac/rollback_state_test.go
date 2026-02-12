package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/git"
)

func TestRollbackCmd_RestoresState(t *testing.T) {
	// Setup Workspace
	tmpDir, err := os.MkdirTemp("", "recac-rollback-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change to tmpDir so rollbackCmd picks it up as CWD
	cwd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(cwd)

	// Initialize Git
	gitClient := git.NewClient()
	_, err = gitClient.Run(tmpDir, "init")
	require.NoError(t, err)
	gitClient.Config(tmpDir, "user.email", "test@example.com")
	gitClient.Config(tmpDir, "user.name", "Test User")

	// Initial commit
	gitClient.Run(tmpDir, "commit", "--allow-empty", "-m", "Initial commit")

	// Switch to feature branch
	gitClient.Run(tmpDir, "checkout", "-b", "feature/rollback-test")

	// 1. Create State & Commit (Iteration 1)
	state1 := `{"memory":["iteration1"]}`
	os.MkdirAll(filepath.Join(tmpDir, ".recac", "state"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".recac", "state", "agent_state.json"), []byte(state1), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".agent_state.json"), []byte(state1), 0644)

	gitClient.Run(tmpDir, "add", "-f", filepath.Join(".recac", "state", "agent_state.json"))
	gitClient.Run(tmpDir, "commit", "-m", "chore: progress update (iteration 1)")

	// Get SHA of iteration 1
	sha1, _ := gitClient.CurrentCommitSHA(tmpDir)

	// 2. Modify State & Commit (Iteration 2)
	state2 := `{"memory":["iteration2"]}`
	os.WriteFile(filepath.Join(tmpDir, ".recac", "state", "agent_state.json"), []byte(state2), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".agent_state.json"), []byte(state2), 0644)

	gitClient.Run(tmpDir, "add", "-f", filepath.Join(".recac", "state", "agent_state.json"))
	gitClient.Run(tmpDir, "commit", "-m", "chore: progress update (iteration 2)")

	// 3. Mock askOne
	originalAskOne := askOne
	defer func() { askOne = originalAskOne }()

	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// If Select prompt, choose the first option (which corresponds to Iteration 2 in logs usually, but let's check prompt)
		if selectPrompt, ok := p.(*survey.Select); ok {
			// We want to select "iteration 1".
			// Options are formatted as "%s - %s (%s)"
			// We iterate options to find the one with our SHA.
			for _, opt := range selectPrompt.Options {
				if len(opt) >= 7 && opt[:7] == sha1[:7] {
					// Set response
					*(response.(*string)) = opt
					return nil
				}
			}
			return fmt.Errorf("could not find option for sha %s", sha1)
		}
		// If Confirm prompt, confirm
		if _, ok := p.(*survey.Confirm); ok {
			*(response.(*bool)) = true
			return nil
		}
		return nil
	}

	// 4. Run Rollback
	cmd := rollbackCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	// Execute directly to avoid root command overhead
	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	// Verify output contains success message
	assert.Contains(t, buf.String(), "Agent state restored from time capsule!")

	// 5. Verify State Restored
	content, err := os.ReadFile(filepath.Join(tmpDir, ".agent_state.json"))
	require.NoError(t, err)
	assert.Equal(t, state1, string(content))
}
