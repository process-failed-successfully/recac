package telemetry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// safeBuffer is a simple thread-safe buffer.
type safeBuffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *safeBuffer) String() string {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.String()
}

func (b *safeBuffer) Reset() {
	b.m.Lock()
	defer b.m.Unlock()
	b.b.Reset()
}

// TestNewLogger verifies the NewLogger function's behavior under various configurations.
func TestNewLogger(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "test.log")
	require.NoError(t, err)
	tempFile.Close() // Close it so the logger can open it

	// A file path that is not writable
	unwritableDir := t.TempDir()
	require.NoError(t, os.Chmod(unwritableDir, 0500)) // Read-execute only
	unwritableFile := unwritableDir + "/unwritable.log"

	testCases := []struct {
		name          string
		debug         bool
		logFile       string
		silenceStdout bool
		logMessage    func(l *slog.Logger)
		wantInStdout  string
		wantInLogFile string
		expectError   bool
	}{
		{
			name:          "Debug to Stdout",
			debug:         true,
			logFile:       "",
			silenceStdout: false,
			logMessage:    func(l *slog.Logger) { l.Debug("debug message") },
			wantInStdout:  `"level":"DEBUG","msg":"debug message"`,
			wantInLogFile: "",
		},
		{
			name:          "Info to Stdout with Debug disabled",
			debug:         false,
			logFile:       "",
			silenceStdout: false,
			logMessage:    func(l *slog.Logger) { l.Debug("debug message") },
			wantInStdout:  "",
		},
		{
			name:          "Info to File only",
			debug:         false,
			logFile:       tempFile.Name(),
			silenceStdout: true,
			logMessage:    func(l *slog.Logger) { l.Info("info message") },
			wantInStdout:  "",
			wantInLogFile: `"level":"INFO","msg":"info message"`,
		},
		{
			name:          "Debug to File and Stdout",
			debug:         true,
			logFile:       tempFile.Name(),
			silenceStdout: false,
			logMessage:    func(l *slog.Logger) { l.Debug("multi message") },
			wantInStdout:  `"level":"DEBUG","msg":"multi message"`,
			wantInLogFile: `"level":"DEBUG","msg":"multi message"`,
		},
		{
			name:          "Completely Silenced",
			debug:         true,
			logFile:       "",
			silenceStdout: true,
			logMessage:    func(l *slog.Logger) { l.Debug("should not see this") },
			wantInStdout:  "",
			wantInLogFile: "",
		},
		{
			name:          "Invalid log file",
			debug:         true,
			logFile:       unwritableFile,
			silenceStdout: false, // Will still log error to default logger (stderr)
			logMessage:    func(l *slog.Logger) { l.Info("message") },
			wantInStdout:  `"level":"INFO","msg":"message"`,
			expectError:   true, // The initial setup will log an error
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// The NewLogger function can write to the *default* slog logger if it fails to open a file.
			// To capture that, we need to set the default logger temporarily.
			var defaultLogBuf safeBuffer
			defaultLogger := slog.New(slog.NewJSONHandler(&defaultLogBuf, nil))
			oldDefault := slog.Default()
			slog.SetDefault(defaultLogger)

			// Clean up log file before test
			if tc.logFile != "" && !strings.Contains(tc.logFile, "unwritable") {
				require.NoError(t, os.WriteFile(tc.logFile, []byte{}, 0644))
			}

			// Create the logger instance to be tested
			logger := NewLogger(tc.debug, tc.logFile, tc.silenceStdout)
			tc.logMessage(logger)

			// Restore everything and capture output
			w.Close()
			os.Stdout = oldStdout
			slog.SetDefault(oldDefault)
			var capturedStdout bytes.Buffer
			_, err := io.Copy(&capturedStdout, r)
			require.NoError(t, err)

			// Check stdout
			if tc.wantInStdout != "" {
				assert.Contains(t, capturedStdout.String(), tc.wantInStdout)
			} else {
				assert.Empty(t, capturedStdout.String())
			}

			// Check for the setup error
			if tc.expectError {
				assert.Contains(t, defaultLogBuf.String(), "Failed to open log file")
			}

			// Check log file content
			if tc.logFile != "" && !strings.Contains(tc.logFile, "unwritable") {
				logContent, err := os.ReadFile(tc.logFile)
				require.NoError(t, err)
				if tc.wantInLogFile != "" {
					assert.Contains(t, string(logContent), tc.wantInLogFile)
				} else {
					assert.Empty(t, string(logContent))
				}
			}
		})
	}
}

// mockHandler is a simple handler for testing the multiHandler wrapper.
type mockHandler struct {
	enabled bool
	err     error
	attrs   []slog.Attr
	group   string
	handled bool
}

func (h *mockHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.enabled
}

func (h *mockHandler) Handle(ctx context.Context, record slog.Record) error {
	h.handled = true
	return h.err
}

func (h *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Return a new instance to avoid modifying the original
	return &mockHandler{
		enabled: h.enabled,
		err:     h.err,
		attrs:   append(h.attrs, attrs...),
		group:   h.group,
	}
}

func (h *mockHandler) WithGroup(name string) slog.Handler {
	// Return a new instance to avoid modifying the original
	return &mockHandler{
		enabled: h.enabled,
		err:     h.err,
		attrs:   h.attrs,
		group:   h.group + name,
	}
}

func TestMultiHandler(t *testing.T) {
	ctx := context.Background()
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "test", 0)

	t.Run("Enabled", func(t *testing.T) {
		h1 := &mockHandler{enabled: false}
		h2 := &mockHandler{enabled: true}
		multi := &multiHandler{handlers: []slog.Handler{h1, h2}}
		assert.True(t, multi.Enabled(ctx, slog.LevelInfo))

		h2.enabled = false
		assert.False(t, multi.Enabled(ctx, slog.LevelInfo))
	})

	t.Run("Handle", func(t *testing.T) {
		h1 := &mockHandler{}
		h2 := &mockHandler{}
		multi := &multiHandler{handlers: []slog.Handler{h1, h2}}
		err := multi.Handle(ctx, record)
		assert.NoError(t, err)
		assert.True(t, h1.handled)
		assert.True(t, h2.handled)
	})

	t.Run("Handle returns first error", func(t *testing.T) {
		expectedErr := errors.New("handler error")
		h1 := &mockHandler{err: expectedErr}
		h2 := &mockHandler{} // This one won't be called if the first fails
		multi := &multiHandler{handlers: []slog.Handler{h1, h2}}
		err := multi.Handle(ctx, record)
		assert.Equal(t, expectedErr, err)
		assert.True(t, h1.handled)
		assert.False(t, h2.handled) // Important: execution should stop
	})

	t.Run("WithAttrs", func(t *testing.T) {
		h1 := &mockHandler{}
		h2 := &mockHandler{}
		multi := &multiHandler{handlers: []slog.Handler{h1, h2}}
		attrs := []slog.Attr{slog.String("key", "value")}

		newMultiHandler := multi.WithAttrs(attrs)
		newMulti, ok := newMultiHandler.(*multiHandler)
		require.True(t, ok)

		assert.Len(t, newMulti.handlers, 2)
		assert.Equal(t, attrs, newMulti.handlers[0].(*mockHandler).attrs)
		assert.Equal(t, attrs, newMulti.handlers[1].(*mockHandler).attrs)
	})

	t.Run("WithGroup", func(t *testing.T) {
		h1 := &mockHandler{}
		h2 := &mockHandler{}
		multi := &multiHandler{handlers: []slog.Handler{h1, h2}}
		groupName := "my-group"

		newMultiHandler := multi.WithGroup(groupName)
		newMulti, ok := newMultiHandler.(*multiHandler)
		require.True(t, ok)

		assert.Len(t, newMulti.handlers, 2)
		assert.Equal(t, groupName, newMulti.handlers[0].(*mockHandler).group)
		assert.Equal(t, groupName, newMulti.handlers[1].(*mockHandler).group)
	})
}

func TestLogInfof(t *testing.T) {
	var buf bytes.Buffer
	// Temporarily redirect default logger to our buffer
	originalLogger := slog.Default()
	t.Cleanup(func() { slog.SetDefault(originalLogger) })

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))

	// Case 1: Level is enabled, message should be logged.
	LogInfof("hello %s", "world")
	assert.Contains(t, buf.String(), `"msg":"hello world"`)
	assert.Contains(t, buf.String(), `"level":"INFO"`)

	buf.Reset()

	// Case 2: Level is disabled, nothing should be logged.
	handler = slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelWarn, // A higher level, so Info messages will be ignored.
	})
	slog.SetDefault(slog.New(handler))
	LogInfof("you should not see %s", "this")
	assert.Empty(t, buf.String())
}

func TestInitLogger(t *testing.T) {
	// Redirect stdout to a pipe to capture output
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call InitLogger
	InitLogger(true, "", false) // Debug, no file, stdout enabled

	// Check if the default logger is now a debug logger
	isEnabled := slog.Default().Enabled(context.Background(), slog.LevelDebug)
	assert.True(t, isEnabled, "Default logger should have Debug level enabled")

	// Log a message to see if it comes out on stdout
	slog.Debug("init test")

	// Close the writer and restore stdout
	w.Close()
	os.Stdout = old

	// Read the output from the pipe
	var buf bytes.Buffer
	io.Copy(&buf, r)

	assert.Contains(t, buf.String(), `"level":"DEBUG","msg":"init test"`)
}
