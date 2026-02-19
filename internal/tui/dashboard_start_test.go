package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDashboardModel(t *testing.T) {
	host := "http://localhost:8080"
	model := NewDashboardModel(host)

	assert.Equal(t, host, model.host)
	assert.NotNil(t, model.table)

	// Check columns
	cols := model.table.Columns()
	assert.Len(t, cols, 4)
	assert.Equal(t, "ID", cols[0].Title)
	assert.Equal(t, "Summary", cols[1].Title)
	assert.Equal(t, "Status", cols[2].Title)
	assert.Equal(t, "Duration", cols[3].Title)

	// Check initial state
	assert.Empty(t, model.jobs)
	assert.False(t, model.quitting)
	assert.Nil(t, model.err)
}
