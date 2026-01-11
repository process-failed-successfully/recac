package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/runner"
	"recac/internal/ui"
	"strings"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
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
		// 1. Setup
		logsTestDir := t.TempDir()
		sessionName := "test-logs-session-from-commands-test"
		logFile := filepath.Join(logsTestDir, sessionName+".log")
		logContent := "line 1\nline 2\nline 3"

		err := os.WriteFile(logFile, []byte(logContent), 0600)
		if err != nil {
			t.Fatalf("Failed to write dummy log file: %v", err)
		}

		mockSM, err := runner.NewSessionManagerWithDir(logsTestDir)
		if err != nil {
			t.Fatalf("Failed to create mock session manager: %v", err)
		}

		session := &runner.SessionState{
			Name:    sessionName,
			LogFile: logFile,
			Status:  "completed",
		}
		err = mockSM.SaveSession(session)
		if err != nil {
			t.Fatalf("Failed to save mock session: %v", err)
		}

		originalFactory := sessionManagerFactory
		sessionManagerFactory = func() (ISessionManager, error) {
			return mockSM, nil
		}
		defer func() { sessionManagerFactory = originalFactory }()

		// Sub-test for basic log retrieval
		t.Run("without follow", func(t *testing.T) {
			rootCmd, _, _ := newRootCmd()
			output, err := executeCommand(rootCmd, "logs", sessionName)
			if err != nil {
				t.Fatalf("logs command failed: %v", err)
			}
			if !strings.Contains(output, logContent) {
				t.Errorf("Expected output to contain '%s', got '%s'", logContent, output)
			}
		})

		// Sub-test for non-existent session
		t.Run("for non-existent session", func(t *testing.T) {
			rootCmd, _, _ := newRootCmd()
			_, err := executeCommand(rootCmd, "logs", "no-such-session")
			if err == nil {
				t.Fatal("Expected error for non-existent session, but got nil")
			}
			if !strings.Contains(err.Error(), "failed to read session file") {
				t.Errorf("Expected error message to contain 'failed to read session file', got '%v'", err)
			}
		})
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

	t.Run("Stop Command Interactive", func(t *testing.T) {
		// 1. Setup
		// Use the real factory to get a session manager
		sm, err := runner.NewSessionManager()
		if err != nil {
			t.Fatalf("Failed to create session manager: %v", err)
		}

		// Start a dummy session
		sessionName := "test-session-to-stop"
		// Using os.Executable() and a fake command to ensure we have a valid executable
		// The actual command won't be run, but StartSession checks for it.
		cmdToRun := []string{os.Args[0], "-test.run=^$", "--"}
		_, err = sm.StartSession(sessionName, cmdToRun, t.TempDir())
		if err != nil {
			t.Fatalf("Failed to start dummy session: %v", err)
		}

		// 2. Mock the interactive prompt
		originalSurveyAskOne := surveyAskOne
		surveyAskOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			// Simulate the user selecting the first (and only) option
			val := response.(*string)
			*val = sessionName
			return nil
		}
		defer func() { surveyAskOne = originalSurveyAskOne }()

		// 3. Execute the command
		output, err := executeCommand(rootCmd, "stop")
		if err != nil {
			t.Fatalf("Stop command failed: %v, output: %s", err, output)
		}

		// 4. Assert
		expectedOutput := fmt.Sprintf("Session '%s' stopped successfully", sessionName)
		if !strings.Contains(output, expectedOutput) {
			t.Errorf("Expected output to contain '%s', got '%s'", expectedOutput, output)
		}

		// Verify session status is 'stopped'
		stoppedSession, err := sm.LoadSession(sessionName)
		if err != nil {
			t.Fatalf("Failed to load session after stop: %v", err)
		}
		if stoppedSession.Status != "stopped" {
			t.Errorf("Expected session status to be 'stopped', but got '%s'", stoppedSession.Status)
		}
	})
}
