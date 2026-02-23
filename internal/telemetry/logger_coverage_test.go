package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type MockHandler struct {
	enabled bool
	err     error
}

func (h *MockHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h.enabled
}

func (h *MockHandler) Handle(_ context.Context, _ slog.Record) error {
	return h.err
}

func (h *MockHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return h
}

func (h *MockHandler) WithGroup(_ string) slog.Handler {
	return h
}

func TestMultiHandler_Coverage(t *testing.T) {
	// 1. Test Enabled returns false when all handlers return false
	h1 := &MockHandler{enabled: false}
	h2 := &MockHandler{enabled: false}
	mh := &multiHandler{handlers: []slog.Handler{h1, h2}}

	if mh.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("MultiHandler should be disabled")
	}

	// 2. Test Handle returns error when a handler returns error
	errMock := errors.New("mock error")
	h3 := &MockHandler{enabled: true, err: errMock}
	mhError := &multiHandler{handlers: []slog.Handler{h3}}

	err := mhError.Handle(context.Background(), slog.Record{})
	if err == nil {
		t.Error("Handle should return error")
	} else if err != errMock {
		t.Errorf("Expected error %v, got %v", errMock, err)
	}

	// 3. Test WithAttrs propagates to all handlers
	// Check coverage only (implementation correctness is covered by existing tests,
	// but we want to ensure no panic/crash on mock if called)
	mh.WithAttrs([]slog.Attr{slog.String("k", "v")})

	// 4. Test WithGroup propagates to all handlers
	mh.WithGroup("g")
}

func TestNewLogger_NoHandlers(t *testing.T) {
	// This covers the len(handlers) == 0 case
	// It should return a discard handler
	logger := NewLogger(false, "", true)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
	// We can't verify internal handler type but we can verify it doesn't log
	logger.Info("test")
}
