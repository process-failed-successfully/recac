package orchestrator

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type multiTestPoller struct {
	items []WorkItem
	err   error
	// To track calls
	updatedStatus map[string]string
}

func (m *multiTestPoller) Poll(ctx context.Context, logger *slog.Logger) ([]WorkItem, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return a copy to simulate fresh fetch
	return append([]WorkItem{}, m.items...), nil
}

func (m *multiTestPoller) UpdateStatus(ctx context.Context, item WorkItem, status string, comment string) error {
	if m.updatedStatus == nil {
		m.updatedStatus = make(map[string]string)
	}
	m.updatedStatus[item.ID] = status
	return nil
}

func TestMultiPoller_Poll(t *testing.T) {
	mp := NewMultiPoller()

	p1 := &multiTestPoller{
		items: []WorkItem{{ID: "1", Summary: "A"}},
	}
	p2 := &multiTestPoller{
		items: []WorkItem{{ID: "2", Summary: "B"}},
	}

	mp.AddPoller("jira", p1)
	mp.AddPoller("github", p2)

	items, err := mp.Poll(context.Background(), slog.Default())
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Verify sources are set correctly
	var found1, found2 bool
	for _, item := range items {
		if item.ID == "1" {
			assert.Equal(t, "jira", item.Source)
			found1 = true
		}
		if item.ID == "2" {
			assert.Equal(t, "github", item.Source)
			found2 = true
		}
	}
	assert.True(t, found1)
	assert.True(t, found2)
}

func TestMultiPoller_Poll_PartialFailure(t *testing.T) {
	mp := NewMultiPoller()

	p1 := &multiTestPoller{
		items: []WorkItem{{ID: "1", Summary: "A"}},
	}
	p2 := &multiTestPoller{
		err: errors.New("fail"),
	}

	mp.AddPoller("jira", p1)
	mp.AddPoller("github", p2)

	items, err := mp.Poll(context.Background(), slog.Default())
	// Should return items from p1, and no error (as per implementation preference for partial success)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "1", items[0].ID)
	assert.Equal(t, "jira", items[0].Source)
}

func TestMultiPoller_Poll_AllFailure(t *testing.T) {
	mp := NewMultiPoller()

	p1 := &multiTestPoller{
		err: errors.New("fail1"),
	}
	p2 := &multiTestPoller{
		err: errors.New("fail2"),
	}

	mp.AddPoller("jira", p1)
	mp.AddPoller("github", p2)

	items, err := mp.Poll(context.Background(), slog.Default())
	require.Error(t, err)
	assert.Nil(t, items)
}

func TestMultiPoller_UpdateStatus(t *testing.T) {
	mp := NewMultiPoller()

	p1 := &multiTestPoller{}
	p2 := &multiTestPoller{}

	mp.AddPoller("jira", p1)
	mp.AddPoller("github", p2)

	// Update item from jira
	item1 := WorkItem{ID: "1", Source: "jira"}
	err := mp.UpdateStatus(context.Background(), item1, "Done", "Fixed")
	require.NoError(t, err)
	assert.Equal(t, "Done", p1.updatedStatus["1"])
	assert.Empty(t, p2.updatedStatus)

	// Update item from github
	item2 := WorkItem{ID: "2", Source: "github"}
	err = mp.UpdateStatus(context.Background(), item2, "Closed", "Merged")
	require.NoError(t, err)
	assert.Equal(t, "Closed", p2.updatedStatus["2"])

	// Update item from unknown source
	item3 := WorkItem{ID: "3", Source: "unknown"}
	err = mp.UpdateStatus(context.Background(), item3, "Done", "")
	require.Error(t, err)
}
