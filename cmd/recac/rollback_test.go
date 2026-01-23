package main

import (
	"errors"
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

	t.Run("Resolve Workspace CWD", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-cwd")
		defer os.RemoveAll(tempDir)

		cwd, _ := os.Getwd()
		defer os.Chdir(cwd)
		os.Chdir(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }

		mockGit.RepoExistsFunc = func(path string) bool {
			// Check if path matches tempDir (CWD)
			absPath, _ := filepath.Abs(path)
			absTemp, _ := filepath.Abs(tempDir)
			return absPath == absTemp
		}
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"sha123456789|msg|time"}, nil
		}
		mockGit.CheckoutFunc = func(repoPath, commitOrBranch string) error { return nil }
		mockGit.CheckoutNewBranchFunc = func(directory, branch string) error { return nil }

		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			switch v := p.(type) {
			case *survey.Select:
				*(response.(*string)) = v.Options[0]
				return nil
			case *survey.Confirm:
				*(response.(*bool)) = true
				return nil
			}
			return nil
		}

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		// Run without session arg
		output, err := executeCommand(rootCmd, "rollback")
		require.NoError(t, err)
		assert.Contains(t, output, "Analyzing workspace")
	})

	t.Run("Resolve Workspace Interactive Selection", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-inter")
		defer os.RemoveAll(tempDir)

		cwd, _ := os.Getwd()
		defer os.Chdir(cwd)
		os.Chdir(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }

		// Simulate CWD is NOT a git repo
		mockGit.RepoExistsFunc = func(path string) bool {
			absPath, _ := filepath.Abs(path)
			absTemp, _ := filepath.Abs(tempDir)
			if absPath == absTemp {
				return false
			}
			// But the session workspace IS a git repo
			return true
		}

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		sessionDir := filepath.Join(tempDir, "session_dir")
		os.Mkdir(sessionDir, 0755)
		mockSM.StartSession("interactive-session", "goal", []string{}, sessionDir)

		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"sha123456789|msg|time"}, nil
		}
		mockGit.CheckoutFunc = func(repoPath, commitOrBranch string) error { return nil }
		mockGit.CheckoutNewBranchFunc = func(directory, branch string) error { return nil }

		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			switch v := p.(type) {
			case *survey.Select:
				if v.Message == "Select a session to rollback:" {
					// Select the session
					for _, opt := range v.Options {
						if filepath.Base(opt) == "interactive-session (running)" || filepath.Base(opt) == "interactive-session (completed)" || filepath.Base(opt) == "interactive-session (failed)" || filepath.Base(opt) == "interactive-session (created)" {
							*(response.(*string)) = opt
							return nil
						}
						// Depending on mock implementation of status
						if opt == "interactive-session (running)" {
							*(response.(*string)) = opt
							return nil
						}
					}
					// Fallback
					*(response.(*string)) = v.Options[0]
					return nil
				}
				if v.Message == "Select a checkpoint to rollback to:" {
					*(response.(*string)) = v.Options[0]
					return nil
				}
			case *survey.Confirm:
				*(response.(*bool)) = true
				return nil
			}
			return nil
		}

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		output, err := executeCommand(rootCmd, "rollback")
		require.NoError(t, err)
		assert.Contains(t, output, "Analyzing workspace")
	})

	t.Run("Error: Repo Check Fail", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-err")
		defer os.RemoveAll(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }
		mockGit.RepoExistsFunc = func(path string) bool { return false } // Fail

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session-err", "goal", []string{}, tempDir)

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		_, err := executeCommand(rootCmd, "rollback", "test-session-err")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "workspace is not a git repository")
	})

	t.Run("Error: Log Fail", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-log-err")
		defer os.RemoveAll(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return nil, errors.New("git log failed")
		}

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session-log-err", "goal", []string{}, tempDir)

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		_, err := executeCommand(rootCmd, "rollback", "test-session-log-err")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to search git log")
	})

	t.Run("Error: Checkout Fail", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-co-err")
		defer os.RemoveAll(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"sha123456789|msg|time"}, nil
		}
		mockGit.CheckoutFunc = func(repoPath, commitOrBranch string) error {
			return errors.New("checkout failed")
		}

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session-co-err", "goal", []string{}, tempDir)

		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			switch p.(type) {
			case *survey.Select:
				*(response.(*string)) = p.(*survey.Select).Options[0]
			case *survey.Confirm:
				*(response.(*bool)) = true
			}
			return nil
		}

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		_, err := executeCommand(rootCmd, "rollback", "test-session-co-err")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to checkout checkpoint")
	})

	t.Run("Error: CheckoutNewBranch Fail", func(t *testing.T) {
		tempDir, _ := os.MkdirTemp("", "recac-rollback-nb-err")
		defer os.RemoveAll(tempDir)

		mockGit := &MockGitClient{}
		gitClientFactory = func() IGitClient { return mockGit }
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentBranchFunc = func(path string) (string, error) { return "main", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"sha123456789|msg|time"}, nil
		}
		mockGit.CheckoutFunc = func(repoPath, commitOrBranch string) error { return nil }
		mockGit.CheckoutNewBranchFunc = func(directory, branch string) error {
			return errors.New("new branch failed")
		}

		mockSM := NewMockSessionManager()
		sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		mockSM.StartSession("test-session-nb-err", "goal", []string{}, tempDir)

		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			switch p.(type) {
			case *survey.Select:
				*(response.(*string)) = p.(*survey.Select).Options[0]
			case *survey.Confirm:
				*(response.(*bool)) = true
			}
			return nil
		}

		rollbackCmd.SetOut(nil)
		rollbackCmd.SetErr(nil)
		_, err := executeCommand(rootCmd, "rollback", "test-session-nb-err")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to reset branch")
	})
}
