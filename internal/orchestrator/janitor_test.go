package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/mock"
)

func TestJanitor_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockDockerClient)
	janitor := NewJanitor(logger, client, "recac-orchestrator", 10*time.Minute, 1*time.Hour)

	ctx := context.Background()
	now := time.Now()

	// Containers:
	// 1. Old and running (should remove)
	// 2. Young and running (keep)
	// 3. Young and exited (remove)
	// 4. Old and exited (remove)

	c1 := types.Container{ID: "c1", Names: []string{"/old-running"}, State: "running", Created: now.Add(-2 * time.Hour).Unix()}
	c2 := types.Container{ID: "c2", Names: []string{"/young-running"}, State: "running", Created: now.Add(-30 * time.Minute).Unix()}
	c3 := types.Container{ID: "c3", Names: []string{"/young-exited"}, State: "exited", Created: now.Add(-30 * time.Minute).Unix()}
	c4 := types.Container{ID: "c4", Names: []string{"/old-exited"}, State: "exited", Created: now.Add(-2 * time.Hour).Unix()}

	client.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		// Verify project label filter
		values := opts.Filters.Get("label")
		for _, v := range values {
			if v == "created-by=recac-orchestrator" {
				return true
			}
		}
		return false
	})).Return([]types.Container{c1, c2, c3, c4}, nil)

	// Expectations for removal
	client.On("RemoveContainer", ctx, "c1", true).Return(nil)
	client.On("RemoveContainer", ctx, "c3", true).Return(nil)
	client.On("RemoveContainer", ctx, "c4", true).Return(nil)

	// c2 should NOT be removed

	err := janitor.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	client.AssertExpectations(t)
	client.AssertNotCalled(t, "RemoveContainer", ctx, "c2", mock.Anything)
}
