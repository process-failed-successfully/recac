package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"testing"
)

type mockHandler struct {
	enabled bool
	err     error
}

func (m *mockHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return m.enabled
}

func (m *mockHandler) Handle(ctx context.Context, record slog.Record) error {
	return m.err
}

func (m *mockHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return m
}

func (m *mockHandler) WithGroup(name string) slog.Handler {
	return m
}

func TestMultiHandler_Coverage(t *testing.T) {
	ctx := context.Background()
	testErr := errors.New("handle error")

	tests := []struct {
		name          string
		handlers      []slog.Handler
		checkEnabled  bool
		wantEnabled   bool
		checkHandle   bool
		wantHandleErr error
	}{
		{
			name: "Enabled_AllDisabled",
			handlers: []slog.Handler{
				&mockHandler{enabled: false},
				&mockHandler{enabled: false},
			},
			checkEnabled: true,
			wantEnabled:  false,
		},
		{
			name: "Handle_Error",
			handlers: []slog.Handler{
				&mockHandler{err: nil},
				&mockHandler{err: testErr},
			},
			checkHandle:   true,
			wantHandleErr: testErr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mh := &multiHandler{handlers: tt.handlers}

			if tt.checkEnabled {
				got := mh.Enabled(ctx, slog.LevelInfo)
				if got != tt.wantEnabled {
					t.Errorf("Enabled() = %v, want %v", got, tt.wantEnabled)
				}
			}

			if tt.checkHandle {
				err := mh.Handle(ctx, slog.Record{})
				if !errors.Is(err, tt.wantHandleErr) {
					t.Errorf("Handle() error = %v, want %v", err, tt.wantHandleErr)
				}
			}
		})
	}
}
