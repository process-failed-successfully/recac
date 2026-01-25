package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"recac/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type MockGitClientCleanup struct {
	MockGitClient
}

// Ensure MockGitClientCleanup satisfies IGitClient
var _ IGitClient = (*MockGitClientCleanup)(nil)

func TestGitCleanupCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	origRunTUI := runGitCleanupTUI
	defer func() {
		gitClientFactory = origGitFactory
		runGitCleanupTUI = origRunTUI
	}()

	mockGit := new(MockGitClientCleanup)
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// Change to temp dir
	tempDir, _ := os.MkdirTemp("", "recac-cleanup-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	t.Run("Not a Git Repo", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return false }

		cmd := gitCleanupCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "not a git repository")
		}
	})

	t.Run("List Branches Error", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.RunFunc = func(path string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "for-each-ref" {
				return "", fmt.Errorf("git error")
			}
			return "", nil
		}

		cmd := gitCleanupCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to list branches")
		}
	})

	t.Run("No Other Branches", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.RunFunc = func(path string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "for-each-ref" {
				now := time.Now().Format(time.RFC3339)
				return fmt.Sprintf("main|%s|Alice", now), nil
			}
			if len(args) > 0 && args[0] == "branch" && args[1] == "--merged" {
				return "main", nil
			}
			return "", nil
		}
		mockGit.CurrentBranchFunc = func(path string) (string, error) {
			return "main", nil
		}

		cmd := gitCleanupCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		// Capture stdout to check message
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, outBuf.String(), "No other local branches found")
	})

	t.Run("Delete Selected Branches", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentBranchFunc = func(path string) (string, error) {
			return "main", nil
		}

		// Mock branches
		mockGit.RunFunc = func(path string, args ...string) (string, error) {
			if len(args) > 0 && args[0] == "for-each-ref" {
				now := time.Now().Format(time.RFC3339)
				// main, feature-1, feature-2
				return fmt.Sprintf("main|%s|Alice\nfeature-1|%s|Bob\nfeature-2|%s|Charlie", now, now, now), nil
			}
			if len(args) > 0 && args[0] == "branch" && args[1] == "--merged" {
				return "main\nfeature-1", nil
			}
			return "", nil
		}

		// Mock TUI to select feature-1
		runGitCleanupTUI = func(m tea.Model) (tea.Model, error) {
			// m is the initial model.
			cleanupModel, ok := m.(ui.GitCleanupModel)
			if !ok {
				return nil, fmt.Errorf("invalid model type")
			}

			// Simulate user interaction:
			// Select feature-1 (index 0, as main is skipped)
			cleanupModel, _ = updateCleanupModel(cleanupModel, tea.KeyMsg{Type: tea.KeySpace})
			// Confirm
			cleanupModel, _ = updateCleanupModel(cleanupModel, tea.KeyMsg{Type: tea.KeyEnter})
			// Yes
			cleanupModel, _ = updateCleanupModel(cleanupModel, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

			return cleanupModel, nil
		}

		// Track deletions
		deleted := []string{}
		mockGit.DeleteLocalBranchFunc = func(repoPath, branch string) error {
			deleted = append(deleted, branch)
			return nil
		}

		cmd := gitCleanupCmd
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)

		assert.Contains(t, deleted, "feature-1")
		assert.NotContains(t, deleted, "feature-2")
		assert.Contains(t, outBuf.String(), "Deleted feature-1")
	})
}

// Helper to update model with type assertion
func updateCleanupModel(m ui.GitCleanupModel, msg tea.Msg) (ui.GitCleanupModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(ui.GitCleanupModel), cmd
}
