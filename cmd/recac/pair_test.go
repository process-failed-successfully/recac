package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ThreadSafeBuffer is a thread-safe wrapper around bytes.Buffer
type ThreadSafeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *ThreadSafeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *ThreadSafeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

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
	var outBuf, errBuf ThreadSafeBuffer
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

	// Wait for startup (Watching message)
	assert.Eventually(t, func() bool {
		return strings.Contains(outBuf.String(), "Watching")
	}, 1*time.Second, 10*time.Millisecond, "Timed out waiting for startup")

	// Trigger event
	select {
	case mockWatcher.events <- fsnotify.Event{
		Name: filePath,
		Op:   fsnotify.Write,
	}:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out sending event")
	}

	// Verify output
	assert.Eventually(t, func() bool {
		return strings.Contains(outBuf.String(), "Detected change") &&
			strings.Contains(outBuf.String(), "LGTM")
	}, 2*time.Second, 10*time.Millisecond, "Timed out waiting for analysis result")

	// Test "feedback" case
	mockAgent.Response = "Found a bug on line 1."

	// Trigger another event
	select {
	case mockWatcher.events <- fsnotify.Event{
		Name: filePath,
		Op:   fsnotify.Write,
	}:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out sending second event")
	}

	// Verify feedback output
	assert.Eventually(t, func() bool {
		return strings.Contains(outBuf.String(), "Feedback for test.go") &&
			strings.Contains(outBuf.String(), "Found a bug on line 1.")
	}, 2*time.Second, 10*time.Millisecond, "Timed out waiting for feedback")

	// Cancel context to stop command
	cancel()

	// Wait for command to finish
	select {
	case err = <-errChan:
		assert.NoError(t, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for command to exit")
	}
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
