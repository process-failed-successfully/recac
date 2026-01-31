package ui

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckGitIdentityDiagnostic(t *testing.T) {
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	t.Run("Git Identity Configured", func(t *testing.T) {
		execCommand = func(name string, arg ...string) *exec.Cmd {
			// Simulate git config returning success
			return exec.Command("echo", "some-value")
		}

		diag := checkGitIdentityDiagnostic()
		assert.Equal(t, "Git Identity", diag.Name)
		assert.Equal(t, "PASS", diag.Status)
	})

	t.Run("Git Identity Missing", func(t *testing.T) {
		execCommand = func(name string, arg ...string) *exec.Cmd {
			// Simulate failure (exit 1)
			cmd := exec.Command("false")
			return cmd
		}

		diag := checkGitIdentityDiagnostic()
		assert.Equal(t, "Git Identity", diag.Name)
		assert.Equal(t, "FAIL", diag.Status)
		assert.True(t, diag.CanAutoFix)
		assert.Equal(t, "fix_git_identity", diag.FixID)
	})
}

func TestDiagnose(t *testing.T) {
	// We need to mock execLookPath and others to prevent flaky tests depending on host env
	origExecLookPath := execLookPath
	origViperConfigFileUsed := viperConfigFileUsed
	defer func() {
		execLookPath = origExecLookPath
		viperConfigFileUsed = origViperConfigFileUsed
	}()

	// Mock dependencies finding
	execLookPath = func(file string) (string, error) {
		return "/usr/bin/" + file, nil
	}

	// Mock config file missing
	viperConfigFileUsed = func() string { return "" }

	diags := Diagnose()
	assert.NotEmpty(t, diags)

	// Verify Config check failed
	foundConfigCheck := false
	for _, d := range diags {
		if d.Name == "Configuration" {
			foundConfigCheck = true
			assert.Equal(t, "FAIL", d.Status)
			assert.True(t, d.CanAutoFix)
		}
	}
	assert.True(t, foundConfigCheck)
}

// Helper to mock LookPath failure
func mockLookPathFail(file string) (string, error) {
	return "", os.ErrNotExist
}
