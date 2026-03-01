package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigEditCmd(t *testing.T) {
	// Setup temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("key: value\n"), 0644)
	require.NoError(t, err)

	// Mock viper.ConfigFileUsed
	origViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string { return configFile }
	defer func() { viperConfigFileUsed = origViperConfigFileUsed }()

	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var executedCmd string
	var executedArgs []string
	execCommand = func(name string, arg ...string) *exec.Cmd {
		executedCmd = name
		executedArgs = arg
		// Return a command that does nothing but succeeds
		return exec.Command("true")
	}

	// Set EDITOR environment variable
	origEditor, editorSet := os.LookupEnv("EDITOR")
	os.Setenv("EDITOR", "my-test-editor")
	defer func() {
		if editorSet {
			os.Setenv("EDITOR", origEditor)
		} else {
			os.Unsetenv("EDITOR")
		}
	}()

	// Execute command via root to ensure context
	cmd := rootCmd
	cmd.SetArgs([]string{"config", "edit"})
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	err = cmd.Execute()

	require.NoError(t, err)
	require.Equal(t, "my-test-editor", executedCmd)
	require.Equal(t, []string{configFile}, executedArgs)
	require.Contains(t, outBuf.String(), "Configuration saved.")
}

func TestConfigEditCmdNoFile(t *testing.T) {
	// Need to make sure we don't accidentally overwrite real config
	originalCwd, _ := os.Getwd()
	tempDir := t.TempDir()
	os.Chdir(tempDir)
	defer os.Chdir(originalCwd)

	// Mock viper.ConfigFileUsed
	origViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string { return "" }
	defer func() { viperConfigFileUsed = origViperConfigFileUsed }()

	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var executedCmd string
	var executedArgs []string
	execCommand = func(name string, arg ...string) *exec.Cmd {
		executedCmd = name
		executedArgs = arg
		return exec.Command("true")
	}

	origEditor, editorSet := os.LookupEnv("EDITOR")
	os.Setenv("EDITOR", "my-test-editor")
	defer func() {
		if editorSet {
			os.Setenv("EDITOR", origEditor)
		} else {
			os.Unsetenv("EDITOR")
		}
	}()

	// Execute command via root
	cmd := rootCmd
	cmd.SetArgs([]string{"config", "edit"})
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	err := cmd.Execute()

	require.NoError(t, err)
	require.Equal(t, "my-test-editor", executedCmd)
	require.Equal(t, []string{"config.yaml"}, executedArgs)
	require.Contains(t, outBuf.String(), "No configuration file found.")
	require.Contains(t, outBuf.String(), "Configuration saved.")
}

func TestConfigEditCmdNoEditor(t *testing.T) {
	// Setup temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("key: value\n"), 0644)
	require.NoError(t, err)

	// Mock viper.ConfigFileUsed
	origViperConfigFileUsed := viperConfigFileUsed
	viperConfigFileUsed = func() string { return configFile }
	defer func() { viperConfigFileUsed = origViperConfigFileUsed }()

	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	var executedCmd string
	var executedArgs []string
	execCommand = func(name string, arg ...string) *exec.Cmd {
		executedCmd = name
		executedArgs = arg
		return exec.Command("true")
	}

	// Unset EDITOR environment variable
	origEditor, editorSet := os.LookupEnv("EDITOR")
	os.Unsetenv("EDITOR")
	defer func() {
		if editorSet {
			os.Setenv("EDITOR", origEditor)
		}
	}()

	// Execute command via root
	cmd := rootCmd
	cmd.SetArgs([]string{"config", "edit"})
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)
	err = cmd.Execute()

	require.NoError(t, err)
	require.Equal(t, "vim", executedCmd)
	require.Equal(t, []string{configFile}, executedArgs)
	require.Contains(t, outBuf.String(), "Configuration saved.")
}
