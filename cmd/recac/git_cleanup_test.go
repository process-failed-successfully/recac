package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGitCleanupCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	defer func() { gitClientFactory = origGitFactory }()

	// We use the shared MockGitClient from test_helpers_test.go
	mockGit := &MockGitClient{}
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// Change to temp dir
	tempDir, _ := os.MkdirTemp("", "recac-git-cleanup-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	t.Run("Not a Git Repo", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return false }

		cmd := gitCleanupCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})

	t.Run("Dry Run - Detect Merged and Stale", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.CurrentBranchFunc = func(dir string) (string, error) { return "main", nil }

		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			cmdStr := strings.Join(args, " ")
			if strings.Contains(cmdStr, "--merged=HEAD") {
				return "feature/merged\nmain", nil
			}
			if strings.Contains(cmdStr, "committerdate") {
				// feature/stale is old
				oldDate := time.Now().AddDate(0, 0, -60).Format("2006-01-02 15:04:05 -0700")
				// feature/active is new
				newDate := time.Now().Format("2006-01-02 15:04:05 -0700")
				return fmt.Sprintf("%s|feature/stale\n%s|feature/active", oldDate, newDate), nil
			}
			return "", nil
		}

		cleanupDryRun = true
		cleanupForce = true // To avoid interactive prompt

		cmd := gitCleanupCmd
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)

		out := outBuf.String()
		assert.Contains(t, out, "[Dry Run] Would delete: feature/merged")
		assert.Contains(t, out, "[Dry Run] Would delete: feature/stale")
		assert.NotContains(t, out, "feature/active")
	})

	t.Run("Force Delete", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.CurrentBranchFunc = func(dir string) (string, error) { return "main", nil }

		deleted := make(map[string]bool)
		mockGit.DeleteLocalBranchFunc = func(dir, branch string) error {
			deleted[branch] = true
			return nil
		}

		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			cmdStr := strings.Join(args, " ")
			if strings.Contains(cmdStr, "--merged=HEAD") {
				return "feature/merged", nil
			}
			return "", nil
		}

		cleanupDryRun = false
		cleanupForce = true
		cleanupMergedOnly = false

		cmd := gitCleanupCmd
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)

		assert.True(t, deleted["feature/merged"])
		assert.Contains(t, outBuf.String(), "Deleted feature/merged")
	})

	t.Run("Merged Only Flag", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.CurrentBranchFunc = func(dir string) (string, error) { return "main", nil }

		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			cmdStr := strings.Join(args, " ")
			if strings.Contains(cmdStr, "--merged=HEAD") {
				return "feature/merged", nil
			}
			if strings.Contains(cmdStr, "committerdate") {
				return "2020-01-01 00:00:00 +0000|feature/stale", nil
			}
			return "", nil
		}

		deleted := make(map[string]bool)
		mockGit.DeleteLocalBranchFunc = func(dir, branch string) error {
			deleted[branch] = true
			return nil
		}

		cleanupDryRun = false
		cleanupForce = true
		cleanupMergedOnly = true

		cmd := gitCleanupCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.NoError(t, err)

		assert.True(t, deleted["feature/merged"])
		assert.False(t, deleted["feature/stale"], "stale branch should not be deleted with merged-only flag")
	})
}
