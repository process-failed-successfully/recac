package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockJanitorClient struct {
	mock.Mock
}

func (m *MockJanitorClient) ListCandidates(ctx context.Context) ([]Candidate, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Candidate), args.Error(1)
}

func (m *MockJanitorClient) Remove(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func TestJanitor_Cleanup(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockJanitorClient)
	ctx := context.Background()

	// Setup candidates
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	newTime := now.Add(-1 * time.Hour)

	candidates := []Candidate{
		{
			ID:        "old-container",
			CreatedAt: oldTime,
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-1",
			},
		},
		{
			ID:        "new-container",
			CreatedAt: newTime,
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-2",
			},
		},
		{
			ID:        "manual-container",
			CreatedAt: oldTime,
			Labels: map[string]string{
				"created-by": "manual",
			},
		},
	}

	client.On("ListCandidates", ctx).Return(candidates, nil)

	// Expect removal of old-container
	client.On("Remove", ctx, "old-container").Return(nil)

	// Janitor setup
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	client.AssertExpectations(t)
	// Ensure new-container and manual-container were NOT removed
	client.AssertNotCalled(t, "Remove", ctx, "new-container")
	client.AssertNotCalled(t, "Remove", ctx, "manual-container")
}

func TestJanitor_Cleanup_DryRun(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockJanitorClient)
	ctx := context.Background()

	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)

	candidates := []Candidate{
		{
			ID:        "old-container",
			CreatedAt: oldTime,
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
			},
		},
	}

	client.On("ListCandidates", ctx).Return(candidates, nil)

	// Janitor setup with dryRun=true
	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, true)

	err := janitor.Cleanup(ctx)
	assert.NoError(t, err)

	// Expect NO removal calls
	client.AssertNotCalled(t, "Remove")
}

func TestJanitor_Cleanup_ListError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := new(MockJanitorClient)
	ctx := context.Background()

	client.On("ListCandidates", ctx).Return(nil, errors.New("list failed"))

	janitor := NewJanitor(logger, client, 1*time.Minute, 24*time.Hour, false)

	err := janitor.Cleanup(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list failed")
}
