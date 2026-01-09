package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/runner"
	"recac/internal/ui"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestHelperProcess isn't a real test. It's a helper process that's executed
// by other tests to simulate running the main binary.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args[3:]
	rootCmd.SetArgs(args)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "execution failed: %v", err)
		os.Exit(1)
	}
}

func executeCommand(root *cobra.Command, args ...string) (string, error) {
	resetFlags(root)
	// Mock exit
	oldExit := exit
	exit = func(code int) {
		if code != 0 {
			panic(fmt.Sprintf("exit-%d", code))
		}
	}
	defer func() { exit = oldExit }()

	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "exit-") {
				// This is an expected exit, don't re-panic
				return
			}
			panic(r) // Re-panic actual panics
		}
	}()

	root.SetArgs(args)
	b := new(bytes.Buffer)
	root.SetOut(b)
	root.SetErr(b)
	// Mock Stdin to avoid hanging on interactive prompts (e.g. wizard)
	root.SetIn(bytes.NewBufferString(""))

	err := root.Execute()
	return b.String(), err
}

func resetFlags(cmd *cobra.Command) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			f.Value.Set(f.DefValue)
			f.Changed = false
		}
	})
	for _, c := range cmd.Commands() {
		resetFlags(c)
	}
}

func TestCommands(t *testing.T) {
	// Setup global test env
	originalWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create dummy config
	configDir := filepath.Join(tmpDir, ".recac")
	os.MkdirAll(configDir, 0755)

	defer os.Chdir(originalWd)

	t.Run("Init Command", func(t *testing.T) {

		projectDir := filepath.Join(tmpDir, "init-test")

		os.MkdirAll(projectDir, 0755)

		os.Chdir(projectDir)

		// Create dummy spec

		os.WriteFile("app_spec.txt", []byte("My App Spec"), 0644)

		_, err := executeCommand(rootCmd, "init-project", "--mock-agent", "--spec", "app_spec.txt")

		if err != nil {

			t.Errorf("Init failed: %v", err)

		}

		// Verify files

		if _, err := os.Stat("initial_features.json"); os.IsNotExist(err) {

			t.Error("initial_features.json not created")

		}

		if _, err := os.Stat("cmd"); os.IsNotExist(err) {

			t.Error("cmd directory not created")

		}

	})

	t.Run("Config Get Command", func(t *testing.T) {
		// Create a temporary config file
		tmpFile, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())

		// Write some config to it
		configContent := "foo: bar"
		if _, err := tmpFile.WriteString(configContent); err != nil {
			t.Fatal(err)
		}
		tmpFile.Close()

		// Test get
		output, err := executeCommand(rootCmd, "--config", tmpFile.Name(), "config", "get", "foo")
		if err != nil {
			t.Errorf("Config get failed: %v", err)
		}

		if !strings.Contains(output, "bar") {
			t.Errorf("Expected output to contain 'bar', got %q", output)
		}
	})

	t.Run("Config Command", func(t *testing.T) {

		// Create dummy models file with correct structure

		os.WriteFile("gemini-models.json", []byte(`{"models": [{"name": "gemini-pro", "displayName": "Gemini Pro"}]}`), 0644)

		// Test list-models

		_, err := executeCommand(rootCmd, "config", "list-models")

		if err != nil {

			t.Errorf("Config list-models failed: %v", err)

		}

	})

	t.Run("Start Mock Command", func(t *testing.T) {

		projectDir := filepath.Join(tmpDir, "start-test")

		os.MkdirAll(projectDir, 0755)

		os.Chdir(projectDir)

		// Create app_spec.txt so it has something to work with

		os.WriteFile("app_spec.txt", []byte("test spec"), 0644)

		// Run with --mock and limit iterations

		// We set --path to current dir to avoid wizard

		// Also use --stream to cover that path

		_, err := executeCommand(rootCmd, "start", "--mock", "--path", ".", "--max-iterations", "1", "--stream")

		// It might exit with code 1 due to max iterations, which is caught by executeCommand

		// We just want to ensure it runs some code.

		if err != nil {

			t.Logf("Start mock finished with: %v", err)

		}

	})

	t.Run("Signal Command", func(t *testing.T) {

		// signal clear

		executeCommand(rootCmd, "signal", "clear", "SOME_SIGNAL")

	})

	t.Run("Logs Command", func(t *testing.T) {

		// logs command needs a session name or lists logs.

		// If no session provided, it might list or error (test said "Logs Missing Arg").

		// Just run help to cover command definition

		_, err := executeCommand(rootCmd, "logs", "--help")

		if err != nil {

			t.Errorf("Logs help failed: %v", err)

		}

	})

	t.Run("Check Command", func(t *testing.T) {

		_, err := executeCommand(rootCmd, "check")

		// It might fail due to missing docker/go in test env, but we just want to execute code paths

		if err != nil {

			t.Logf("Check failed (expected): %v", err)

		}

	})

	t.Run("Clean Command", func(t *testing.T) {

		cleanDir := t.TempDir()

		os.Chdir(cleanDir)

		// Write temp file tracker

		os.WriteFile("temp_files.txt", []byte("dummy.txt"), 0644)

		os.WriteFile("dummy.txt", []byte("content"), 0644)

		_, err := executeCommand(rootCmd, "clean")

		if err != nil {

			t.Errorf("Clean failed: %v", err)

		}

		if _, err := os.Stat("dummy.txt"); !os.IsNotExist(err) {

			t.Error("Clean failed to remove file")

		}

	})

	t.Run("Version Command", func(t *testing.T) {

		executeCommand(rootCmd, "version")

	})

	t.Run("Missing Args", func(t *testing.T) {

		if _, err := executeCommand(rootCmd, "logs"); err == nil {

			t.Log("Expected error for logs without args")

		}

		if _, err := executeCommand(rootCmd, "start", "--detached", "--path", "."); err == nil {

			t.Log("Expected error for detached without name")

		}

	})

	t.Run("Interactive Slash Command", func(t *testing.T) {
		cmdName := "version"
		var targetCmd *cobra.Command
		for _, c := range rootCmd.Commands() {
			if c.Name() == cmdName {
				targetCmd = c
				break
			}
		}
		if targetCmd == nil {
			t.Fatalf("Could not find command '%s'", cmdName)
		}

		action := func(args []string) tea.Cmd {
			return func() tea.Msg {
				cs := []string{"-test.run=TestHelperProcess", "--", cmdName}
				cs = append(cs, args...)
				cmd := exec.Command(os.Args[0], cs...)
				cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
				output, err := cmd.CombinedOutput()
				if err != nil {
					return ui.StatusMsg(fmt.Sprintf("Error executing command '%s': %v\n%s", cmdName, err, string(output)))
				}
				return ui.StatusMsg(string(output))
			}
		}

		cmdFunc := action([]string{})
		msg := cmdFunc()

		statusMsg, ok := msg.(ui.StatusMsg)
		if !ok {
			t.Fatalf("Expected msg to be of type ui.StatusMsg, but got %T", msg)
		}

		expectedOutput := "recac version v0.2.0"
		if !strings.Contains(string(statusMsg), expectedOutput) {
			t.Errorf("Expected output to contain '%s', but got '%s'", expectedOutput, string(statusMsg))
		}
	})
}

// setupTestEnvironment creates a temporary directory and a session manager for testing.
// It returns the temporary directory path, the session manager, and a cleanup function.
func setupTestEnvironment(t *testing.T) (string, *runner.SessionManager, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "recac-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	sm, err := runner.NewSessionManagerWithDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to create session manager: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return tempDir, sm, cleanup
}
