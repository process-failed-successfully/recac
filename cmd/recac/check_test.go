package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake exec command handler for Check tests
func TestCheckHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_CHECK_HELPER_PROCESS") != "1" {
		return
	}
	// Check args to decide outcome
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, subargs := args[0], args[1:]
	switch cmd {
	case "docker":
		if len(subargs) > 0 && subargs[0] == "info" {
			if os.Getenv("MOCK_DOCKER_FAIL") == "1" {
				os.Exit(1)
			}
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func TestCheckCmd(t *testing.T) {
	// Isolate CWD
	origWd, _ := os.Getwd()
	tmpWd := t.TempDir()
	os.Chdir(tmpWd)
	defer os.Chdir(origWd)

	// Setup overrides
	oldLookPath := execLookPath
	oldExecCommand := execCommand
	defer func() {
		execLookPath = oldLookPath
		execCommand = oldExecCommand
	}()

	// Mock LookPath
	execLookPath = func(file string) (string, error) {
		if file == "go" {
			if os.Getenv("MOCK_GO_FAIL") == "1" {
				return "", fmt.Errorf("not found")
			}
			return "/usr/bin/go", nil
		}
		return "", fmt.Errorf("not found")
	}

	// Mock Command
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCheckHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_CHECK_HELPER_PROCESS=1"}
		if os.Getenv("MOCK_DOCKER_FAIL") == "1" {
			cmd.Env = append(cmd.Env, "MOCK_DOCKER_FAIL=1")
		}
		return cmd
	}

	t.Run("All Checks Pass", func(t *testing.T) {
		// Mock Config in CWD so initConfig finds it
		err := os.WriteFile("config.yaml", []byte(""), 0644)
		require.NoError(t, err)
		defer os.Remove("config.yaml")

		viper.Reset()

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(checkCmd)
		checkCmd.Flags().Set("fix", "false")

		output, err := executeCommand(root, "check")
		assert.NoError(t, err)
		assert.Contains(t, output, "All checks passed")
	})

	t.Run("Config Missing No Fix", func(t *testing.T) {
		viper.Reset()
		oldCfg := cfgFile
		cfgFile = "/non/existent/config.yaml"
		defer func() { cfgFile = oldCfg }()

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(checkCmd)
		checkCmd.Flags().Set("fix", "false")

		output, _ := executeCommand(root, "check")
		// Since ReadInConfig fails, ConfigFileUsed might be empty or the set file.
		// If empty, it says "not found". If set, "does not exist".
		// We accept both.
		assert.Contains(t, output, "Config: config file")
		assert.Contains(t, output, "Some checks failed")
	})

	t.Run("Config Missing With Fix", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.Setenv("HOME", tmpDir)
		defer os.Unsetenv("HOME")

		configFile := filepath.Join(tmpDir, ".recac.yaml")
		viper.Reset()
		// Do not set config file, so it defaults to home dir

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(checkCmd)
		checkCmd.Flags().Set("fix", "true")

		output, err := executeCommand(root, "check", "--fix")
		assert.NoError(t, err)
		assert.Contains(t, output, "Config fixed")
		assert.Contains(t, output, "All checks passed")
		assert.FileExists(t, configFile)
	})

	t.Run("Go Missing", func(t *testing.T) {
		os.Setenv("MOCK_GO_FAIL", "1")
		defer os.Unsetenv("MOCK_GO_FAIL")

		// Valid config
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(""), 0644)
		require.NoError(t, err)
		viper.Reset()
		viper.SetConfigFile(configFile)

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(checkCmd)
		checkCmd.Flags().Set("fix", "false")

		output, _ := executeCommand(root, "check")
		assert.Contains(t, output, "Go: go binary not found")
		assert.Contains(t, output, "Some checks failed")
	})

	t.Run("Docker Missing", func(t *testing.T) {
		os.Setenv("MOCK_DOCKER_FAIL", "1")
		defer os.Unsetenv("MOCK_DOCKER_FAIL")

		// Valid config
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.yaml")
		err := os.WriteFile(configFile, []byte(""), 0644)
		require.NoError(t, err)
		viper.Reset()
		viper.SetConfigFile(configFile)

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(checkCmd)
		checkCmd.Flags().Set("fix", "false")

		output, _ := executeCommand(root, "check")
		assert.Contains(t, output, "Docker: exit status 1")
		assert.Contains(t, output, "Some checks failed")
	})
}
