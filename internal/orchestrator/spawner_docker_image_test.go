package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type SyncHandler struct {
	slog.Handler
	logged chan struct{}
	once   *sync.Once
}

func (h *SyncHandler) Handle(ctx context.Context, r slog.Record) error {
	if r.Message == "failed to load session for final update" {
		h.once.Do(func() {
			close(h.logged)
		})
	}
	return h.Handler.Handle(ctx, r)
}

func (h *SyncHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SyncHandler{
		Handler: h.Handler.WithAttrs(attrs),
		logged:  h.logged,
		once:    h.once,
	}
}

func (h *SyncHandler) WithGroup(name string) slog.Handler {
	return &SyncHandler{
		Handler: h.Handler.WithGroup(name),
		logged:  h.logged,
		once:    h.once,
	}
}

func (h *SyncHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.Handler.Enabled(ctx, level)
}

func TestDockerSpawner_Spawn_ImageFlag(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	mockPoller := new(MockPoller)

	logged := make(chan struct{})
	handler := &SyncHandler{
		Handler: slog.NewTextHandler(io.Discard, nil),
		logged:  logged,
		once:    &sync.Once{},
	}
	logger := slog.New(handler)

	imageName := "custom-image:v1.2.3"
	spawner := NewDockerSpawner(logger, mockDocker, imageName, "test-proj", mockPoller, "provider", "model", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{
		ID:      "TICKET-1",
		RepoURL: "https://github.com/test/repo",
	}

	ctx := context.Background()

	// Mock expectations
	mockDocker.On("RunContainer", ctx, imageName, mock.AnythingOfType("string"), mock.Anything, mock.Anything, "").Return("container123", nil)
	mockSM.On("SaveSession", mock.Anything).Return(nil)

	done := make(chan struct{})
	mockSM.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
		close(done)
	}).Return(nil, assert.AnError)

	execCalled := make(chan string, 1)

	// We match "Anything" for arguments so we catch the call, then inspect it in Run
	mockDocker.On("Exec", mock.Anything, "container123", mock.Anything).Run(func(args mock.Arguments) {
		cmd := args.Get(2).([]string)
		// cmd is ["/bin/sh", "-c", "actual command"]
		execCalled <- cmd[2]
	}).Return("output", nil)

	err := spawner.Spawn(ctx, item)
	assert.NoError(t, err)

	select {
	case cmdStr := <-execCalled:
		t.Logf("Captured Command: %s", cmdStr)
		assert.Contains(t, cmdStr, "--image", "Command should contain --image flag")
		assert.Contains(t, cmdStr, imageName, "Command should contain the correct image name")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for Exec call")
	}

	select {
	case <-done:
		// Success (LoadSession called)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for LoadSession call")
	}

	select {
	case <-logged:
		// Success (LoadSession returned and error logged)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for error log")
	}
}
