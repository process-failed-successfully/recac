package ui

import (
	"errors"
	"recac/internal/orchestrator"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrchestratorDashboard_Update(t *testing.T) {
	observer := NewOrchestratorObserver()
	monitor := NewMonitorDashboardModel(ActionCallbacks{})
	model := NewOrchestratorDashboardModel(observer, monitor)

    // Test Init
    cmd := model.Init()
    assert.NotNil(t, cmd)

	// Test PollStart
	newModel, _ := model.Update(PollStartMsg{})
	m := newModel.(OrchestratorDashboardModel)
	assert.Equal(t, "Polling...", m.status)

	// Test PollEnd (Success)
	newModel, _ = m.Update(PollEndMsg{Count: 5, Err: nil})
	m = newModel.(OrchestratorDashboardModel)
	assert.Contains(t, m.status, "Poll Success")
	assert.Contains(t, m.status, "5 items")
	assert.NotEmpty(t, m.events)
    assert.Contains(t, m.events[0], "Found 5 items")

	// Test PollEnd (Failure)
	newModel, _ = m.Update(PollEndMsg{Count: 0, Err: errors.New("fail")})
	m = newModel.(OrchestratorDashboardModel)
	assert.Contains(t, m.status, "Poll Failed")
	assert.Contains(t, m.events[0], "Poll Failed")

	// Test SpawnStart
	item := orchestrator.WorkItem{ID: "TEST-1"}
	newModel, _ = m.Update(SpawnStartMsg{Item: item})
	m = newModel.(OrchestratorDashboardModel)
	assert.Contains(t, m.events[0], "Spawning agent for TEST-1")

    // Test SpawnEnd (Success)
	newModel, _ = m.Update(SpawnEndMsg{Item: item, Err: nil})
	m = newModel.(OrchestratorDashboardModel)
	assert.Contains(t, m.events[0], "Spawned agent for TEST-1")
}
