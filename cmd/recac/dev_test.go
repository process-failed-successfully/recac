package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectDevCommand(t *testing.T) {
	tests := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name:     "Go project",
			files:    []string{"go.mod"},
			expected: "go test ./...",
		},
		{
			name:     "Node project",
			files:    []string{"package.json"},
			expected: "npm test",
		},
		{
			name:     "Make project",
			files:    []string{"Makefile"},
			expected: "make",
		},
		{
			name:     "Python project",
			files:    []string{"requirements.txt"},
			expected: "pytest",
		},
		{
			name:     "Unknown project",
			files:    []string{"foo.txt"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			for _, file := range tt.files {
				f, err := os.Create(filepath.Join(tmpDir, file))
				require.NoError(t, err)
				f.Close()
			}

			cmd := detectDevCommand(tmpDir)
			assert.Equal(t, tt.expected, cmd)
		})
	}
}

func TestParseExtensions(t *testing.T) {
	tests := []struct {
		name     string
		flagExt  string
		cmd      string
		expected []string
	}{
		{
			name:     "Explicit flag",
			flagExt:  ".go, .js",
			cmd:      "anything",
			expected: []string{".go", ".js"},
		},
		{
			name:     "Explicit flag without dots",
			flagExt:  "py, rb",
			cmd:      "anything",
			expected: []string{".py", ".rb"},
		},
		{
			name:     "Inferred Go",
			flagExt:  "",
			cmd:      "go test",
			expected: []string{".go", ".mod"},
		},
		{
			name:     "Inferred Node",
			flagExt:  "",
			cmd:      "npm test",
			expected: []string{".js", ".ts", ".json"},
		},
		{
			name:     "Inferred Python",
			flagExt:  "",
			cmd:      "pytest",
			expected: []string{".py"},
		},
		{
			name:     "Inferred Make",
			flagExt:  "",
			cmd:      "make",
			expected: []string{".go", ".c", ".cpp", ".h", ".rs"},
		},
		{
			name:     "Unknown command",
			flagExt:  "",
			cmd:      "echo hello",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exts := parseExtensions(tt.flagExt, tt.cmd)
			assert.Equal(t, tt.expected, exts)
		})
	}
}

func TestShouldTrigger(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		exts     []string
		expected bool
	}{
		{
			name:     "Match extension",
			path:     "main.go",
			exts:     []string{".go"},
			expected: true,
		},
		{
			name:     "No match extension",
			path:     "main.go",
			exts:     []string{".js"},
			expected: false,
		},
		{
			name:     "Empty extensions (trigger all)",
			path:     "main.go",
			exts:     []string{},
			expected: true,
		},
		{
			name:     "Match one of many",
			path:     "test.ts",
			exts:     []string{".js", ".ts"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldTrigger(tt.path, tt.exts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExecuteDevCommand(t *testing.T) {
	// Swap execCommand with mock
	originalExecCommand := devExecCommand
	defer func() { devExecCommand = originalExecCommand }()

	var executedCmd string
	var executedArgs []string

	devExecCommand = func(name string, arg ...string) *exec.Cmd {
		executedCmd = name
		executedArgs = arg
		// Create a dummy command that just echoes
		return exec.Command("echo", "mock")
	}

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	executeDevCommand("go test ./...")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Equal(t, "go", executedCmd)
	assert.Equal(t, []string{"test", "./..."}, executedArgs)
	assert.Contains(t, output, "Running...")
	assert.Contains(t, output, "Passed")
}

func TestExecuteDevCommand_Fail(t *testing.T) {
	// Swap execCommand with mock
	originalExecCommand := devExecCommand
	defer func() { devExecCommand = originalExecCommand }()

	devExecCommand = func(name string, arg ...string) *exec.Cmd {
		// Create a command that fails
		return exec.Command("false")
	}

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	executeDevCommand("fail command")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "Failed")
}

func TestDevAddRecursiveWatch(t *testing.T) {
	// Setup a temporary directory structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	ignoredDir := filepath.Join(tmpDir, "node_modules")
	hiddenDir := filepath.Join(tmpDir, ".git")

	require.NoError(t, os.Mkdir(subDir, 0755))
	require.NoError(t, os.Mkdir(ignoredDir, 0755))
	require.NoError(t, os.Mkdir(hiddenDir, 0755))

	watcher, err := fsnotify.NewWatcher()
	require.NoError(t, err)
	defer watcher.Close()

	// We can't easily inspect the watcher's internal state (watched directories)
	// directly without using private fields or reflection, which is brittle.
	// Instead, we trust that filepath.Walk works and verify logic by ensuring no error is returned.
	// A more robust test would trigger an event in a subdir and see if the watcher catches it,
	// but that involves timing and OS-specific behavior.
	//
	// However, we can verify that the walk function does NOT return error.

	err = devAddRecursiveWatch(watcher, tmpDir)
	assert.NoError(t, err)

	// To verify exclusions, we'd need to mock fsnotify.Watcher.Add, but it's a struct method.
	// We could create a wrapper interface for Watcher, but that changes production code significantly.
	// Given the constraints, just ensuring it runs without error on a structure is a decent baseline.

	// Let's try to verify by creating a file in subdir and waiting for event
	// Only if the watcher was correctly added will this work.

	// Start a goroutine to read events
	eventDetected := make(chan bool)
	go func() {
		select {
		case <-watcher.Events:
			eventDetected <- true
		case <-time.After(500 * time.Millisecond):
			eventDetected <- false
		}
	}()

	// Create file in subdir
	time.Sleep(50 * time.Millisecond) // Give watcher time to set up
	f, err := os.Create(filepath.Join(subDir, "test.txt"))
	require.NoError(t, err)
	f.Close()

	// Verify event was detected (implies subdir was watched)
	detected := <-eventDetected
	assert.True(t, detected, "Watcher should detect changes in subdirectories")
}

func TestRunDev_Integration(t *testing.T) {
	// Setup temp dir and files
	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	err := os.WriteFile(goMod, []byte("module test"), 0644)
	require.NoError(t, err)

	// Override devExecCommand to mock execution
	originalExecCommand := devExecCommand
	defer func() { devExecCommand = originalExecCommand }()

	devExecCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "mock")
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	// Create a cancellable context
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Configure flags (global vars in dev.go, so restore after)
	oldWatchDir := devWatchDir
	oldCmdFlag := devCmdFlag
	oldDebounce := devDebounce
	defer func() {
		devWatchDir = oldWatchDir
		devCmdFlag = oldCmdFlag
		devDebounce = oldDebounce
	}()

	devWatchDir = tmpDir
	devCmdFlag = "" // Auto-detect
	devDebounce = 10 * time.Millisecond

	// Execute runDev with context
	// devCmd struct is global, we can invoke runDev directly with context
	cmd := devCmd
	cmd.SetContext(ctx) // Use SetContext instead of WithContext
	err = runDev(cmd, []string{})

	// Check results
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	require.NoError(t, err)
	assert.Contains(t, output, "Auto-detected command: go test ./...")
	assert.Contains(t, output, "Watching extensions: [.go .mod]")
	assert.Contains(t, output, "Watching")
}
