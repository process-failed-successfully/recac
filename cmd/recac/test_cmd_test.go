package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent for testing
type TestCmdMockAgent struct {
	mock.Mock
}

func (m *TestCmdMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *TestCmdMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func (m *TestCmdMockAgent) GetState() interface{} {
	return nil
}

func TestRunTest_ExplicitArgs(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock execCommand to capture 'go test'
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" && arg[0] == "test" {
			// Verify args
			expected := []string{"test", "-v", "pkg/a"}
			assert.Equal(t, expected, arg)
			return exec.Command("echo", "ok")
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run
	output, err := executeCommand(cmd, "test", "pkg/a")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Running tests for 1 packages")
	assert.Contains(t, output, "ok")
}

func TestRunTest_Impacted(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock getGitDiffFilesFunc
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"file.go"}, nil
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	// Mock identifyPackagesFunc
	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/affected"}, map[string]bool{"pkg/changed": true}, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock execCommand for go test
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" && arg[0] == "test" {
			// Verify args
			expected := []string{"test", "-v", "pkg/affected"}
			assert.Equal(t, expected, arg)
			return exec.Command("echo", "PASS")
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run without args (triggers impact analysis)
	output, err := executeCommand(cmd, "test")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Analyzing impact")
	assert.Contains(t, output, "Running tests for 1 packages")
	assert.Contains(t, output, "PASS")
}

func TestRunTest_DiagnoseFailure(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock getGitDiffFilesFunc
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"fail.go"}, nil
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	// Mock identifyPackagesFunc
	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/fail"}, nil, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock Agent
	mockAgent := new(TestCmdMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).
		Return("Diagnosis: You broke it.", nil).
		Run(func(args mock.Arguments) {
			cb := args.Get(2).(func(string))
			cb("Diagnosis: You broke it.")
		})

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand to FAIL
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" {
			c := exec.Command("sh", "-c", "echo 'FAIL output'; exit 1")
			return c
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run
	output, err := executeCommand(cmd, "test")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, output, "Tests failed")
	assert.Contains(t, output, "Diagnosing failure with AI")
	assert.Contains(t, output, "Diagnosis: You broke it")
}

func TestRunTest_DiagnoseFailure_AgentError(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock getGitDiffFilesFunc
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"fail.go"}, nil
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	// Mock identifyPackagesFunc
	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/fail"}, nil, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock Agent to ERROR
	mockAgent := new(TestCmdMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).
		Return("", fmt.Errorf("API error"))

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand to FAIL
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" {
			c := exec.Command("sh", "-c", "echo 'FAIL output'; exit 1")
			return c
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run
	output, err := executeCommand(cmd, "test")

	// Assert
	assert.Error(t, err)
	assert.Contains(t, output, "Tests failed")
	assert.Contains(t, output, "Diagnosing failure with AI")
	assert.Contains(t, err.Error(), "agent failed during diagnosis")
}

func TestRunTest_ImpactAnalysisErrors(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// 1. getGitDiffFilesFunc error
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return nil, fmt.Errorf("git error")
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	output, err := executeCommand(cmd, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get changed files")
	assert.Contains(t, err.Error(), "git error")

	// 2. No changed files
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{}, nil
	}
	output, err = executeCommand(cmd, "test")
	assert.NoError(t, err)
	assert.Contains(t, output, "No changed files found")

	// 3. identifyPackagesFunc generic error
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"foo.go"}, nil
	}
	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return nil, nil, fmt.Errorf("analysis failed")
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	output, err = executeCommand(cmd, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "impact analysis failed")
	assert.Contains(t, err.Error(), "analysis failed")

	// 4. No Go packages found (specific error)
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return nil, nil, fmt.Errorf("No Go packages found for the specified files")
	}

	output, err = executeCommand(cmd, "test")
	assert.NoError(t, err)
	assert.Contains(t, output, "No affected Go packages found")
}

func TestShouldIgnoreDir(t *testing.T) {
	tests := []struct {
		path   string
		expect bool
	}{
		{".git", true},
		{".idea", true},
		{"node_modules", true},
		{"vendor", true},
		{"dist", true},
		{"build", true},
		{"internal", false},
		{"cmd", false},
		{"pkg/foo", false},
		{".", false},
	}

	for _, tc := range tests {
		got := shouldIgnoreDir(tc.path)
		assert.Equal(t, tc.expect, got, "Path: %s", tc.path)
	}
}

func TestRunWatchLoop(t *testing.T) {
	// Setup Mocks
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"file.go"}, nil
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/test"}, nil, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	execCommand = func(name string, arg ...string) *exec.Cmd {
		return exec.Command("echo", "ok")
	}
	defer func() { execCommand = exec.Command }()

	// Channels
	events := make(chan fsnotify.Event)
	errors := make(chan error)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := rootCmd
	resetFlags(cmd)

	// Start loop in background
	done := make(chan error)
	go func() {
		done <- runWatchLoop(ctx, events, errors, cmd, []string{})
	}()

	// 1. Send ignored event (chmod)
	events <- fsnotify.Event{Name: "file.go", Op: fsnotify.Chmod}
	time.Sleep(50 * time.Millisecond) // Give it time to process (should do nothing)

	// 2. Send ignored file (.tmp)
	events <- fsnotify.Event{Name: "file.tmp", Op: fsnotify.Write}
	time.Sleep(50 * time.Millisecond)

	// 3. Send valid event
	// Note: It uses time.AfterFunc, so we need to wait > 500ms
	events <- fsnotify.Event{Name: "file.go", Op: fsnotify.Write}

	// Capture output? We can check stdout if we want, but tricky async.
	// We just want to ensure it doesn't crash and hopefully runs.
	// Since we mocked execCommand, we won't see much action unless we spy on it.
	// But coverage should be hit.

	time.Sleep(600 * time.Millisecond) // Wait for debounce

	// 4. Send error
	errors <- fmt.Errorf("watcher error")
	time.Sleep(50 * time.Millisecond)

	// Stop
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("runWatchLoop did not exit")
	}
}

func TestAddRecursiveWatch(t *testing.T) {
	watcher, err := fsnotify.NewWatcher()
	assert.NoError(t, err)
	defer watcher.Close()

	// Create temp dir structure
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	nodeModules := filepath.Join(tmpDir, "node_modules")

	err = os.Mkdir(subDir, 0755)
	assert.NoError(t, err)
	err = os.Mkdir(nodeModules, 0755)
	assert.NoError(t, err)

	err = addRecursiveWatch(watcher, tmpDir)
	assert.NoError(t, err)

	// Since we can't inspect the watcher list easily, we assume success if no error
	// and if the logic was exercised.
	// We can verify shouldIgnoreDir logic is correct via TestShouldIgnoreDir
}

func TestShouldIgnoreFile(t *testing.T) {
	tests := []struct {
		path   string
		expect bool
	}{
		{"foo.go", false},
		{"foo.tmp", true},
		{"temp_file.tmp", true},
		{"README.md", false},
	}

	for _, tc := range tests {
		got := shouldIgnoreFile(tc.path)
		assert.Equal(t, tc.expect, got, "Path: %s", tc.path)
	}
}
