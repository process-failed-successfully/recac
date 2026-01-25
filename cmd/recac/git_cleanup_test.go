package main

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MockGitClientCleanup struct {
	MockGitClient
}

func TestGitCleanupCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	defer func() { gitClientFactory = origGitFactory }()

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
}
