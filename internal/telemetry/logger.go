package telemetry

import (
	"log/slog"
	"os"
)

// InitLogger configures the default logger.
func InitLogger(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// LogDebug logs a debug message.
func LogDebug(msg string, args ...any) {
	slog.Debug(msg, args...)
}

// LogInfo logs an info message.
func LogInfo(msg string, args ...any) {
	slog.Info(msg, args...)
}

// LogError logs an error message.
func LogError(msg string, err error, args ...any) {
	slog.Error(msg, append(args, "error", err)...)
}
