package main

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCheckCmdDetailed(t *testing.T) {
	// Setup mocks
	originalLookPath := lookPath
	originalExecCommand := execCommand
	defer func() {
		lookPath = originalLookPath
		execCommand = originalExecCommand
	}()

	// Setup config
	tmpConfig := t.TempDir() + "/config.yaml"
	os.WriteFile(tmpConfig, []byte{}, 0644)
	viper.SetConfigFile(tmpConfig)

	t.Run("All checks pass", func(t *testing.T) {
		lookPath = func(file string) (string, error) {
			if file == "go" || file == "kubectl" {
				return "/usr/bin/" + file, nil
			}
			return "", fmt.Errorf("not found")
		}

		execCommand = func(name string, arg ...string) *exec.Cmd {
			// Used by checkDocker
			return exec.Command("true")
		}

		output, err := executeCommand(rootCmd, "check")
		assert.NoError(t, err)
		assert.Contains(t, output, "✅ Config found")
		assert.Contains(t, output, "✅ Go installed")
		assert.Contains(t, output, "✅ Docker running")
		assert.Contains(t, output, "✅ Kubernetes (kubectl) installed")
		assert.Contains(t, output, "All checks passed!")
	})

	t.Run("Kubernetes missing", func(t *testing.T) {
		lookPath = func(file string) (string, error) {
			if file == "go" {
				return "/usr/bin/go", nil
			}
			return "", fmt.Errorf("not found")
		}

		execCommand = func(name string, arg ...string) *exec.Cmd {
			return exec.Command("true")
		}

		output, err := executeCommand(rootCmd, "check")
		assert.NoError(t, err)

		assert.Contains(t, output, "✅ Config found")
		assert.Contains(t, output, "✅ Go installed")
		assert.Contains(t, output, "✅ Docker running")
		assert.Contains(t, output, "⚠️  Kubernetes: kubectl binary not found in PATH")

		// checkK8s failure does NOT fail the command, so "All checks passed!" is still printed
		// unless logic is changed.
		// "All checks passed! 🚀" is printed if `allPassed` is true.
		// `checkK8s` is not updating `allPassed`.
		assert.Contains(t, output, "All checks passed!")
	})

	t.Run("Docker missing", func(t *testing.T) {
		lookPath = func(file string) (string, error) {
			if file == "go" || file == "kubectl" {
				return "/usr/bin/" + file, nil
			}
			return "", fmt.Errorf("not found")
		}

		execCommand = func(name string, arg ...string) *exec.Cmd {
			// Fail Docker check
			return exec.Command("false")
		}

		output, err := executeCommand(rootCmd, "check")
		assert.NoError(t, err) // executeCommand suppresses exit panic

		assert.Contains(t, output, "❌ Docker: exit status 1")
		assert.Contains(t, output, "Some checks failed")
		assert.NotContains(t, output, "All checks passed!")
	})
}
