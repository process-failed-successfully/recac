package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRollbackCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	origSessionManagerFactory := sessionManagerFactory
	origAskOne := askOne
	defer func() {
		gitClientFactory = origGitFactory
		sessionManagerFactory = origSessionManagerFactory
		askOne = origAskOne
	}()

	t.Run("Rollback Successful", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-test")
		defer os.RemoveAll(tempDir)

		// Create dummy agent state
		os.WriteFile(filepath.Join(tempDir, ".agent_state.json"), []byte("{}"), 0644)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }

		// Mock session manager to return tempDir
		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session", "goal", []string{}, tempDir)

		// Mock Git Interactions
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentCommitSHAFunc = func(path string) (string, error) { return "current-sha", nil }

		currentBranch := "main"
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return currentBranch, nil }

		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{
				"sha123456789|chore: progress update (iteration 2)|2 hours ago",
				"sha456789012|chore: progress update (iteration 1)|4 hours ago",
			}, nil
		}

		checkedOutSHA := ""
		mockGit.CheckoutFunc = func(repoPath, commitOrBranch string) error {
			checkedOutSHA = commitOrBranch
			return nil
		}

		resetBranch := ""
		mockGit.CheckoutNewBranchFunc = func(directory, branch string) error {
			resetBranch = branch
			return nil
		}

		// Mock Survey
		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			switch v := p.(type) {
			case *survey.Select:
				if v.Message == "Select a checkpoint to rollback to:" {
					// Select the first one
					// Options format: "SHA... - Msg (Time)"
					// Just ensure we pick something that exists
					if len(v.Options) > 0 {
						*(response.(*string)) = v.Options[0]
					}
					return nil
				}
			case *survey.Confirm:
				*(response.(*bool)) = true
				return nil
			}
			return fmt.Errorf("unexpected prompt: %T", p)
		}

		// Run command using helper
		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		output, err := executeCommand(rootCmd, "rollback", "test-session")
		require.NoError(t, err)
		if err != nil {
			fmt.Println(output)
		}

		assert.Equal(t, "sha123456789", checkedOutSHA)
		assert.Equal(t, "main", resetBranch)

		// Verify agent state deleted
		_, err = os.Stat(filepath.Join(tempDir, ".agent_state.json"))
		assert.True(t, os.IsNotExist(err), ".agent_state.json should be deleted")
	})

	t.Run("No Checkpoints Found", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-test-2")
		defer os.RemoveAll(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }

		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{}, nil // Empty log
		}

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session-2", "goal", []string{}, tempDir)

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		output, err := executeCommand(rootCmd, "rollback", "test-session-2")
		require.NoError(t, err)

		assert.Contains(t, output, "No checkpoints found")
	})

	t.Run("Cancelled by User", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-test-3")
		defer os.RemoveAll(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }

		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"sha123456789|msg|time"}, nil
		}

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session-3", "goal", []string{}, tempDir)

		// User cancels at confirmation
		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			switch p.(type) {
			case *survey.Select:
				if len(p.(*survey.Select).Options) > 0 {
					*(response.(*string)) = p.(*survey.Select).Options[0]
				}
				return nil
			case *survey.Confirm:
				*(response.(*bool)) = false // User says No
				return nil
			}
			return nil
		}

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		output, err := executeCommand(rootCmd, "rollback", "test-session-3")
		require.NoError(t, err)
		assert.Contains(t, output, "Rollback cancelled")
	})
}
