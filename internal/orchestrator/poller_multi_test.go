package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockPollerStruct is a concrete implementation of Poller for testing
type MockPollerStruct struct {
	Items    []WorkItem
	PollErr  error
	Updates  map[string]string // ID -> Status
}

func (m *MockPollerStruct) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	return m.Items, m.PollErr
}

func (m *MockPollerStruct) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	if m.Updates == nil {
		m.Updates = make(map[string]string)
	}
	m.Updates[item.ID] = status
	return nil
}

func TestMultiPoller(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	p1 := &MockPollerStruct{
		Items: []WorkItem{{ID: "1", Source: "src1"}},
	}
	p2 := &MockPollerStruct{
		Items: []WorkItem{{ID: "2", Source: "src2"}},
	}

	mp := NewMultiPoller(map[string]Poller{
		"src1": p1,
		"src2": p2,
	})

	// Test Poll
	items, err := mp.Poll(context.Background(), logger)
	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.ElementsMatch(t, []string{"1", "2"}, []string{items[0].ID, items[1].ID})

	// Test UpdateStatus
	err = mp.UpdateStatus(context.Background(), WorkItem{ID: "1", Source: "src1"}, "Done", "")
	assert.NoError(t, err)
	assert.Equal(t, "Done", p1.Updates["1"])
	assert.Empty(t, p2.Updates)

	err = mp.UpdateStatus(context.Background(), WorkItem{ID: "2", Source: "src2"}, "In Progress", "")
	assert.NoError(t, err)
	assert.Equal(t, "In Progress", p2.Updates["2"])

	// Test Unknown Source
	err = mp.UpdateStatus(context.Background(), WorkItem{ID: "3", Source: "unknown"}, "Fail", "")
	assert.Error(t, err)
}

func TestMultiPoller_PartialFailure(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	p1 := &MockPollerStruct{
		PollErr: errors.New("fail"),
	}
	p2 := &MockPollerStruct{
		Items: []WorkItem{{ID: "2", Source: "src2"}},
	}

	mp := NewMultiPoller(map[string]Poller{
		"src1": p1,
		"src2": p2,
	})

	items, err := mp.Poll(context.Background(), logger)
	assert.NoError(t, err) // Should not fail overall
	assert.Len(t, items, 1)
	assert.Equal(t, "2", items[0].ID)
}
