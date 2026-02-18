package orchestrator

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestJanitor_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mockDocker := new(MockDockerClient)
	janitor := NewJanitor(mockDocker, logger, 1*time.Hour, 24*time.Hour)

	ctx := context.Background()
	now := time.Now()

	// Setup containers with long IDs
	idStale1 := "stale-1-0123456789abcdef0123456789abcdef0123456789abcdef"
	idFresh1 := "fresh-1-0123456789abcdef0123456789abcdef0123456789abcdef"
	idStale2 := "stale-2-0123456789abcdef0123456789abcdef0123456789abcdef"

	containers := []types.Container{
		{
			ID:      idStale1,
			Created: now.Add(-25 * time.Hour).Unix(),
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"created-at": now.Add(-25 * time.Hour).Format(time.RFC3339),
			},
		},
		{
			ID:      idFresh1,
			Created: now.Add(-1 * time.Hour).Unix(),
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"created-at": now.Add(-1 * time.Hour).Format(time.RFC3339),
			},
		},
		{
			ID:      idStale2, // Use Created timestamp fallback
			Created: now.Add(-26 * time.Hour).Unix(),
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
			},
		},
	}

	// Expectations

	// We can't match exact Filters object because it contains internal maps
	// But we can match the All: true
	mockDocker.On("ListContainers", ctx, mock.MatchedBy(func(opts container.ListOptions) bool {
		return opts.All == true
	})).Return(containers, nil)

	mockDocker.On("RemoveContainer", ctx, idStale1, true).Return(nil)
	mockDocker.On("RemoveContainer", ctx, idStale2, true).Return(nil)

	// fresh-1 should not be removed

	err := janitor.cleanup(ctx)
	assert.NoError(t, err)

	mockDocker.AssertExpectations(t)
}
