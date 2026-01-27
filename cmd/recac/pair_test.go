package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockFileWatcher implements FileWatcher for testing
type MockFileWatcher struct {
	events chan fsnotify.Event
	errors chan error
}

func (m *MockFileWatcher) Events() <-chan fsnotify.Event {
	return m.events
}

func (m *MockFileWatcher) Errors() <-chan error {
	return m.errors
}

func (m *MockFileWatcher) Add(name string) error {
	return nil
}

func (m *MockFileWatcher) Close() error {
	return nil
}

func (m *MockFileWatcher) AddRecursive(root string) error {
	return nil
}

// PairTestMockAgent implements agent.Agent for testing
type PairTestMockAgent struct {
	Response string
}

func (m *PairTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *PairTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestPairCmd(t *testing.T) {
	// Setup temp dir and file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.go")
	err := os.WriteFile(filePath, []byte("package main\nfunc main() {}"), 0644)
	require.NoError(t, err)

	// Mock watcher
	mockWatcher := &MockFileWatcher{
		events: make(chan fsnotify.Event),
		errors: make(chan error),
	}
	originalWatcherFactory := watcherFactory
	watcherFactory = func() (FileWatcher, error) {
		return mockWatcher, nil
	}
	defer func() { watcherFactory = originalWatcherFactory }()

	// Mock agent
	mockAgent := &PairTestMockAgent{Response: "LGTM"}
	originalAgentClientFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentClientFactory }()

	// Override debounce for speed
	originalDebounce := pairDebounce
	pairDebounce = 10 * time.Millisecond
	defer func() { pairDebounce = originalDebounce }()

	// Create command
	cmd := &cobra.Command{Use: "pair", RunE: runPair}
	var outBuf, errBuf SafeBuffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Create context with cancel
	ctx, cancel := context.WithCancel(context.Background())
	cmd.SetContext(ctx)

	// Run command in goroutine
	errChan := make(chan error)
	go func() {
		errChan <- cmd.ExecuteContext(ctx)
	}()

	// Wait a bit for startup
	time.Sleep(50 * time.Millisecond)

	// Trigger event
	mockWatcher.events <- fsnotify.Event{
		Name: filePath,
		Op:   fsnotify.Write,
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify output
	output := outBuf.String()
	assert.Contains(t, output, "Watching")
	assert.Contains(t, output, "Detected change")
	assert.Contains(t, output, "LGTM")

	// Test "feedback" case
	mockAgent.Response = "Found a bug on line 1."

	// Trigger another event
	mockWatcher.events <- fsnotify.Event{
		Name: filePath,
		Op:   fsnotify.Write,
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	output = outBuf.String()
	assert.Contains(t, output, "Feedback for test.go")
	assert.Contains(t, output, "Found a bug on line 1.")

	// Cancel context to stop command
	cancel()

	// Wait for command to finish
	err = <-errChan
	// ExecuteContext returns error if context canceled? Usually nil if RunE returns nil.
	// runPair returns nil on ctx.Done().
	assert.NoError(t, err)
}

func TestPairCmdRecursiveAdd(t *testing.T) {
	// Setup real fsnotify watcher to test recursive logic (partially)
	// Or just test that AddRecursive calls WalkDir.
	// Since we mocked FSNotifyWatcher, let's test the helper method directly if possible.
	// But FSNotifyWatcher is in main package.

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// We can't easily test real FSNotifyWatcher without real OS events which might be flaky.
	// But we can verify our FSNotifyWatcher.AddRecursive implementation logic by instantiating it.

	w, err := NewFSNotifyWatcher()
	if err != nil {
		t.Skip("Skipping FSNotify test (inotify limit or other system issue)")
	}
	defer w.Close()

	err = w.AddRecursive(tmpDir)
	assert.NoError(t, err)

	// We can't verify what's added easily on the wrapper.
	// But at least it didn't crash.
}
