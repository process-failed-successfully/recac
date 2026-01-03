package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// InitLogger configures the default logger with optional file output.
func InitLogger(debug bool, logFile string) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}

	// Default handler is stdout
	var handlers []slog.Handler
	handlers = append(handlers, slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	// Add file handler if requested
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			handlers = append(handlers, slog.NewJSONHandler(f, &slog.HandlerOptions{
				Level: level,
			}))
		} else {
			slog.Error("Failed to open log file", "path", logFile, "error", err)
		}
	}

	// Use a multi-handler if we have more than one
	var handler slog.Handler
	if len(handlers) > 1 {
		handler = &multiHandler{handlers: handlers}
	} else {
		handler = handlers[0]
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		if err := h.Handle(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
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

// LogInfof logs an info message with formatting.
func LogInfof(format string, args ...any) {
	if slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		slog.Info(fmt.Sprintf(format, args...))
	}
}
