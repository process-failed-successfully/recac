package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock handler to inspect log records
type mockHandler struct {
	mu       sync.Mutex
	records  []slog.Record
	attrs    []slog.Attr
	group    string
	enabled  bool
	handleFn func(slog.Record) error // Optional custom handle logic
}

func (h *mockHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.enabled
}

func (h *mockHandler) Handle(ctx context.Context, record slog.Record) error {
	if h.handleFn != nil {
		return h.handleFn(record)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record)
	return nil
}

func (h *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	newHandler := *h
	newHandler.attrs = append(h.attrs, attrs...)
	return &newHandler
}

func (h *mockHandler) WithGroup(name string) slog.Handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	newHandler := *h
	if newHandler.group == "" {
		newHandler.group = name
	} else {
		newHandler.group = newHandler.group + "." + name
	}
	return &newHandler
}

func (h *mockHandler) getRecords() []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.records
}

func TestMultiHandler(t *testing.T) {
	h1 := &mockHandler{enabled: true}
	h2 := &mockHandler{enabled: true}

	multi := &multiHandler{handlers: []slog.Handler{h1, h2}}

	t.Run("Enabled", func(t *testing.T) {
		assert.True(t, multi.Enabled(context.Background(), slog.LevelInfo))

		h1.enabled = false
		h2.enabled = false
		assert.False(t, multi.Enabled(context.Background(), slog.LevelInfo))
	})

	t.Run("Handle", func(t *testing.T) {
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "test message", 0)
		err := multi.Handle(context.Background(), record)
		assert.NoError(t, err)
		assert.Len(t, h1.getRecords(), 1)
		assert.Len(t, h2.getRecords(), 1)
		assert.Equal(t, "test message", h1.getRecords()[0].Message)
	})

	t.Run("WithAttrs", func(t *testing.T) {
		attrs := []slog.Attr{slog.String("key", "value")}
		handlerWithAttrs := multi.WithAttrs(attrs)

		// Check if the new handler is a multiHandler
		newMulti, ok := handlerWithAttrs.(*multiHandler)
		require.True(t, ok, "WithAttrs should return a *multiHandler")

		// Check if underlying handlers have the attributes
		for _, h := range newMulti.handlers {
			mockH, ok := h.(*mockHandler)
			require.True(t, ok)
			assert.Equal(t, attrs, mockH.attrs)
		}
	})

	t.Run("WithGroup", func(t *testing.T) {
		handlerWithGroup := multi.WithGroup("my-group")

		newMulti, ok := handlerWithGroup.(*multiHandler)
		require.True(t, ok, "WithGroup should return a *multiHandler")

		for _, h := range newMulti.handlers {
			mockH, ok := h.(*mockHandler)
			require.True(t, ok)
			assert.Equal(t, "my-group", mockH.group)
		}
	})
}

func TestNewLogger(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	t.Run("Debug true", func(t *testing.T) {
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		slog.SetDefault(logger)

		slog.Debug("debug message")
		assert.Contains(t, buf.String(), "debug message")
	})

	t.Run("File logging", func(t *testing.T) {
		tmpfile, err := os.CreateTemp("", "test.log")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		logger := NewLogger(false, tmpfile.Name(), true) // Silence stdout
		logger.Info("file message")

		content, err := io.ReadAll(tmpfile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "file message")
	})

	t.Run("Silence stdout", func(t *testing.T) {
		// Capture original stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		logger := NewLogger(false, "", true) // No file, stdout silenced
		logger.Info("should not appear")

		// Restore stdout
		w.Close()
		os.Stdout = old

		var buf bytes.Buffer
		io.Copy(&buf, r)

		assert.Empty(t, buf.String())
	})

	t.Run("No handlers", func(t *testing.T) {
		// This tests the edge case where there are no handlers configured.
		// It should not panic and should effectively discard logs.
		logger := NewLogger(false, "", true) // No file, stdout silenced
		assert.NotNil(t, logger)
		// No easy way to verify discard, but we ensure it doesn't crash.
		logger.Info("this goes to dev/null")
	})
}

func TestLogInfof(t *testing.T) {
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	LogInfof("hello, %s", "world")

	var logOutput map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logOutput)
	require.NoError(t, err)

	assert.Equal(t, "hello, world", logOutput["msg"])
	assert.Equal(t, "INFO", logOutput["level"])
}

func TestNewLogger_FileError(t *testing.T) {
	// Save original logger and restore it after the test
	originalLogger := slog.Default()
	defer slog.SetDefault(originalLogger)

	// Capture log output in a buffer
	var buf bytes.Buffer
	testLogger := slog.New(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(testLogger)

	// Use a path that's intentionally invalid for logging
	invalidPath := filepath.Join(t.TempDir(), "nonexistent/test.log")
	// Call NewLogger, which should log an error to our testLogger
	logger := NewLogger(false, invalidPath, true) // Silence stdout to not interfere
	assert.NotNil(t, logger)

	// Verify that an error was logged to our buffer
	output := buf.String()
	assert.True(t, strings.Contains(output, "Failed to open log file"), "Expected log file error message, got: "+output)
}
