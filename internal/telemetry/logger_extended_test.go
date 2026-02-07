package telemetry

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestNewLogger_MultiHandler(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "test.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Create logger with both stdout and file
	logger := NewLogger(false, tmpPath, false)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	// We can't easily inspect if it's a multiHandler because it's private and wrapped in *Logger.
	// But we can verify it logs to file.
	// Stdout verification is harder without capturing stdout, but we trust NewLogger logic:
	// if !silenceStdout (false) -> add handler
	// if logFile != "" -> add handler
	// if > 1 handler -> multiHandler.

	// To verify it's working as expected, we log something and check file.
	logger.Info("multi handler test")

	content, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(string(content), "multi handler test") {
		t.Error("Log file missing content")
	}

	// We can also test WithGroup and WithAttrs on the returned logger to ensure multiHandler delegates correctly.
	sub := logger.With("component", "test")
	sub.Info("sub logger test")

	content, _ = os.ReadFile(tmpPath)
	if !contains(string(content), `"component":"test"`) {
		t.Error("Log file missing attribute")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || (len(s) > len(substr) && contains(s[1:], substr))
}

func TestMultiHandler_Directly(t *testing.T) {
	// Since multiHandler is not exported, we can only test it if we are in the same package.
	// We are in telemetry package (telemetry_test if separate, but here package telemetry).

	// If we are in `package telemetry`, we can access `multiHandler`.
	// logger_test.go is in `package telemetry`. So we should be too.

	// But NewLogger returns *slog.Logger, which wraps the handler.
	// We can get the handler via reflection or just trust the functional test above.

	// However, we can create a multiHandler directly here since we are in the same package.
	// Wait, test files usually use `package telemetry` or `package telemetry_test`.
	// `logger_test.go` uses `package telemetry`. So we are fine.

	mh := &multiHandler{
		handlers: []slog.Handler{
			slog.NewJSONHandler(os.Stdout, nil),
			slog.NewJSONHandler(os.Stderr, nil),
		},
	}

	// Test WithGroup
	mhGroup := mh.WithGroup("group")
	// Verify it returns a multiHandler
	if _, ok := mhGroup.(*multiHandler); !ok {
		t.Error("WithGroup should return *multiHandler")
	}

	// Test WithAttrs
	mhAttrs := mh.WithAttrs([]slog.Attr{slog.String("key", "val")})
	if _, ok := mhAttrs.(*multiHandler); !ok {
		t.Error("WithAttrs should return *multiHandler")
	}

	// Test Enabled
	if !mh.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("Enabled should return true")
	}
}
