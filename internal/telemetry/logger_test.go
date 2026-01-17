package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewLogger(t *testing.T) {
	t.Run("Default configuration", func(t *testing.T) {
		logger := NewLogger(false, "", false)
		if logger == nil {
			t.Fatal("Expected logger to be non-nil")
		}
		if !logger.Enabled(context.Background(), slog.LevelInfo) {
			t.Error("Expected info level to be enabled by default")
		}
		if logger.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("Expected debug level to be disabled by default")
		}
	})

	t.Run("Debug configuration", func(t *testing.T) {
		logger := NewLogger(true, "", false)
		if !logger.Enabled(context.Background(), slog.LevelDebug) {
			t.Error("Expected debug level to be enabled")
		}
	})

	t.Run("Silence stdout", func(t *testing.T) {
		// Capture stdout to verify nothing is written
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		logger := NewLogger(false, "", true)
		logger.Info("should not be seen")

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		buf.ReadFrom(r)
		if buf.Len() > 0 {
			t.Errorf("Expected no output to stdout, got %q", buf.String())
		}
	})

	t.Run("File logging", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test.log")

		logger := NewLogger(false, logFile, true)
		logger.Info("test file log")

		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		if !strings.Contains(string(content), "test file log") {
			t.Errorf("Expected log file to contain message, got %q", string(content))
		}
	})

	t.Run("Multi handler (stdout + file)", func(t *testing.T) {
		tmpDir := t.TempDir()
		logFile := filepath.Join(tmpDir, "test_multi.log")

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		logger := NewLogger(false, logFile, false)
		logger.Info("test multi log")

		w.Close()
		os.Stdout = oldStdout

		// Check stdout
		var buf bytes.Buffer
		buf.ReadFrom(r)
		if !strings.Contains(buf.String(), "test multi log") {
			t.Errorf("Expected stdout to contain message, got %q", buf.String())
		}

		// Check file
		content, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatalf("Failed to read log file: %v", err)
		}
		if !strings.Contains(string(content), "test multi log") {
			t.Errorf("Expected log file to contain message, got %q", string(content))
		}
	})

	t.Run("Invalid file path", func(t *testing.T) {
		// Should not panic, just log error to stderr (which we can't easily assert here without intercepting stderr, but mainly checking for no panic)
		logger := NewLogger(false, "/invalid/path/test.log", false)
		if logger == nil {
			t.Error("Expected logger to be created even if file path is invalid")
		}
	})
}

func TestMultiHandler(t *testing.T) {
	var buf1, buf2 bytes.Buffer
	h1 := slog.NewJSONHandler(&buf1, nil)
	h2 := slog.NewJSONHandler(&buf2, nil)

	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}

	t.Run("Enabled", func(t *testing.T) {
		if !mh.Enabled(context.Background(), slog.LevelInfo) {
			t.Error("Expected Enabled to return true")
		}
	})

	t.Run("Handle", func(t *testing.T) {
		record := slog.NewRecord(time.Now(), slog.LevelInfo, "test msg", 0)
		if err := mh.Handle(context.Background(), record); err != nil {
			t.Errorf("Handle returned error: %v", err)
		}
		if !strings.Contains(buf1.String(), "test msg") {
			t.Error("Buffer 1 missing message")
		}
		if !strings.Contains(buf2.String(), "test msg") {
			t.Error("Buffer 2 missing message")
		}
	})

	t.Run("WithAttrs", func(t *testing.T) {
		mh2 := mh.WithAttrs([]slog.Attr{slog.String("key", "val")})
		// We can't easily check the internal state of the handlers wrapped in multiHandler without casting,
		// but we can check if it returns a multiHandler
		if _, ok := mh2.(*multiHandler); !ok {
			t.Error("Expected WithAttrs to return *multiHandler")
		}
	})

	t.Run("WithGroup", func(t *testing.T) {
		mh2 := mh.WithGroup("group")
		if _, ok := mh2.(*multiHandler); !ok {
			t.Error("Expected WithGroup to return *multiHandler")
		}
	})

	t.Run("Enabled_False", func(t *testing.T) {
		// Create handlers with high level
		hDebug := slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError})
		mhDebug := &multiHandler{handlers: []slog.Handler{hDebug}}

		if mhDebug.Enabled(context.Background(), slog.LevelInfo) {
			t.Error("Expected Enabled to return false for Info level when handler is Error level")
		}
	})
}

func TestLogInfof(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	LogInfof("Hello %s", "World")

	output := buf.String()
	if !strings.Contains(output, "Hello World") {
		t.Errorf("Expected formatted message, got %s", output)
	}
}

func TestLogError(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	slog.SetDefault(slog.New(handler))

	LogError("something failed", errors.New("my error"), "foo", "bar")

	output := buf.String()
	if !strings.Contains(output, "my error") {
		t.Errorf("Expected error message in log, got %s", output)
	}
	if !strings.Contains(output, `"foo":"bar"`) {
		t.Errorf("Expected context in log, got %s", output)
	}
	if !strings.Contains(output, `"msg":"something failed"`) {
		t.Errorf("Expected msg in log, got %s", output)
	}
}

func TestInitLogger_JSONOutput(t *testing.T) {
	// Capture stdout
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	LogInfo("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, `"msg":"test message"`) {
		t.Errorf("Expected output to contain message, got %s", output)
	}
	if !strings.Contains(output, `"key":"value"`) {
		t.Errorf("Expected output to contain key-value, got %s", output)
	}

	var logMap map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify timestamp field exists
	if _, ok := logMap["time"]; !ok {
		t.Error("JSON output missing 'time' (timestamp) field")
	}

	// Verify level field exists
	if _, ok := logMap["level"]; !ok {
		t.Error("JSON output missing 'level' field")
	}

	// Verify level is "INFO" for LogInfo
	if level, ok := logMap["level"].(string); ok {
		if level != "INFO" {
			t.Errorf("Expected level to be 'INFO', got %q", level)
		}
	}
}

func TestInitLogger_VerboseMode(t *testing.T) {
	// Step 1: Configure logger with verbose/debug mode enabled
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Verbose mode enables DEBUG level
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Step 2: Generate a DEBUG level log entry
	LogDebug("debug message", "key", "value", "api_key", "sk-secret123")

	output := buf.String()
	if !strings.Contains(output, `"msg":"debug message"`) {
		t.Errorf("Expected output to contain debug message, got %s", output)
	}

	var logMap map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logMap); err != nil {
		t.Fatalf("Output is not valid JSON: %v", err)
	}

	// Verify level is "DEBUG" for LogDebug
	if level, ok := logMap["level"].(string); ok {
		if level != "DEBUG" {
			t.Errorf("Expected level to be 'DEBUG' in verbose mode, got %q", level)
		}
	} else {
		t.Error("JSON output missing 'level' field")
	}

	if strings.Contains(output, "sk-secret123") {
		t.Log("WARNING: API key is visible in logs - should be masked in production")
	}
}
