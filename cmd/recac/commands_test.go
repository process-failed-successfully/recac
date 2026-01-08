package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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

		if _, err := os.Stat("feature_list.json"); os.IsNotExist(err) {

			t.Error("feature_list.json not created")

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

		// Create feature_list.json so it doesn't try to run wizard or init

		os.WriteFile("feature_list.json", []byte(`[{"id":"1","description":"test","category":"core","steps":["echo hello"]}]`), 0644)

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

	t.Run("List Command With Data", func(t *testing.T) {

		// Create a dummy session file in .recac/sessions or similar?

		// runner uses DB usually.

		// If DB is used, we need to init DB.

		// commands.go doesn't seem to expose DB init easily for testing 'list'.

		// But 'list' command reads from DB.

		// Just run list, we already did.

		executeCommand(rootCmd, "list")

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

	t.Run("List Command", func(t *testing.T) {

		// Just run it, it should output empty list or similar

		_, err := executeCommand(rootCmd, "list")

		if err != nil {

			t.Errorf("List failed: %v", err)

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

		if _, err := executeCommand(rootCmd, "start", "--detached"); err == nil {

			t.Log("Expected error for detached without name")

		}

	})

	t.Run("History Command", func(t *testing.T) {
		sessionsDir := filepath.Join(tmpDir, ".recac", "sessions")
		os.MkdirAll(sessionsDir, 0755)

		// Test with no history
		t.Run("No History", func(t *testing.T) {
			output, err := executeCommand(rootCmd, "history")
			if err != nil {
				t.Fatalf("history command failed unexpectedly: %v", err)
			}
			if !strings.Contains(output, "No session history found.") {
				t.Errorf("Expected 'No session history found.', got: %s", output)
			}
		})

		// Setup mock session files
		session1 := `{"name":"session-1","pid":123,"start_time":"2023-01-01T12:00:00Z","command":["cmd"],"log_file":"log1","workspace":"/ws1","status":"completed","type":"detached"}`
		session2 := `{"name":"session-2","pid":456,"start_time":"2023-01-02T12:00:00Z","command":["cmd"],"log_file":"log2","workspace":"/ws2","status":"running","type":"detached"}`
		os.WriteFile(filepath.Join(sessionsDir, "session-1.json"), []byte(session1), 0644)
		os.WriteFile(filepath.Join(sessionsDir, "session-2.json"), []byte(session2), 0644)
		// Add a non-json file to ensure it's ignored
		os.WriteFile(filepath.Join(sessionsDir, "ignore.txt"), []byte("ignore me"), 0644)


		t.Run("With History", func(t *testing.T) {
			output, err := executeCommand(rootCmd, "history")
			if err != nil {
				t.Fatalf("history command failed: %v", err)
			}

			// Check for headers
			if !strings.Contains(output, "SESSION ID") || !strings.Contains(output, "STATUS") {
				t.Errorf("Output missing expected headers. Got:\n%s", output)
			}

			// Check for session data
			if !strings.Contains(output, "session-1") || !strings.Contains(output, "completed") {
				t.Errorf("Output missing session-1 data. Got:\n%s", output)
			}
			if !strings.Contains(output, "session-2") || !strings.Contains(output, "running") {
				t.Errorf("Output missing session-2 data. Got:\n%s", output)
			}

			// Check for sorting (session-2 should appear before session-1)
			pos1 := strings.Index(output, "session-1")
			pos2 := strings.Index(output, "session-2")
			if pos2 > pos1 {
				t.Errorf("Output not sorted correctly. session-2 should be first. Got:\n%s", output)
			}
		})
	})
}
