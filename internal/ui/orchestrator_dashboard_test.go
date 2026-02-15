package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestOrchestratorDashboardModel_Update(t *testing.T) {
	model := NewOrchestratorDashboard()

	// Init
	cmd := model.Init()
	assert.NotNil(t, cmd)

	// Update with PollStartMsg
	updatedModel, cmd := model.Update(PollStartMsg{})
	m := updatedModel.(*OrchestratorDashboardModel)
	assert.Equal(t, "Polling...", m.pollStatus)
	assert.NotNil(t, cmd)

	// Update with PollEndMsg
	updatedModel, cmd = model.Update(PollEndMsg{Count: 5, Err: nil})
	m = updatedModel.(*OrchestratorDashboardModel)
	assert.Contains(t, m.pollStatus, "Success")
	assert.Contains(t, m.pollStatus, "5 items")
	assert.NotZero(t, m.lastPollTime)

	// Update with SpawnStartMsg
	item := orchestrator.WorkItem{ID: "TEST-1"}
	updatedModel, cmd = model.Update(SpawnStartMsg{Item: item})
	m = updatedModel.(*OrchestratorDashboardModel)
	assert.Contains(t, m.activeSpawns, "TEST-1")

	// Update with SpawnEndMsg
	updatedModel, cmd = model.Update(SpawnEndMsg{Item: item, Err: nil})
	m = updatedModel.(*OrchestratorDashboardModel)
	assert.NotContains(t, m.activeSpawns, "TEST-1")
}

func TestOrchestratorDashboardModel_Observer(t *testing.T) {
	model := NewOrchestratorDashboard()

	// Test Observer methods push to channel
	go func() {
		model.OnPollStart()
	}()

	msg := <-model.events
	assert.IsType(t, PollStartMsg{}, msg)

	go func() {
		model.OnPollEnd(10, nil)
	}()
	msg = <-model.events
	pollEnd := msg.(PollEndMsg)
	assert.Equal(t, 10, pollEnd.Count)
}
