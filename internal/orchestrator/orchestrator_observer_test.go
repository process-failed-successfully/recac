package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockObserver definition (repeated here or shared if exported)
type TestObserver struct {
	mock.Mock
}

func (m *TestObserver) OnPollStart() {
	m.Called()
}

func (m *TestObserver) OnPollEnd(count int, err error) {
	m.Called(count, err)
}

func (m *TestObserver) OnSpawnStart(item WorkItem) {
	m.Called(item)
}

func (m *TestObserver) OnSpawnEnd(item WorkItem, err error) {
	m.Called(item, err)
}

func TestOrchestrator_ObserverCalls(t *testing.T) {
	// Setup
	poller := newMockPoller([]WorkItem{{ID: "TEST-1", Summary: "Test Item"}})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 10*time.Millisecond)

	obs := &TestObserver{}

	// Expectations
	// We expect PollStart to be called at least once
	obs.On("OnPollStart").Return()

	// We expect PollEnd to return 1 item exactly once
	obs.On("OnPollEnd", 1, nil).Return().Once()

	// We expect subsequent PollEnds to return 0 items
	obs.On("OnPollEnd", 0, nil).Return()

	// We expect SpawnStart/End for the specific item exactly once
	obs.On("OnSpawnStart", mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "TEST-1"
	})).Return().Once()

	obs.On("OnSpawnEnd", mock.MatchedBy(func(item WorkItem) bool {
		return item.ID == "TEST-1"
	}), nil).Return().Once()

	orch.SetObserver(obs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run Orchestrator in background
	go func() {
		_ = orch.Run(ctx, silentLogger)
	}()

	// Wait enough time for at least one poll and spawn
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Verify
	obs.AssertExpectations(t)
}

func TestTUIModel_Update(t *testing.T) {
	model := InitialModel()

	// Test MsgPollStart
	updated, _ := model.Update(MsgPollStart{})
	m := updated.(*TUIModel)
	assert.Equal(t, "Polling...", m.PollStatus)

	// Test MsgPollEnd
	updated, _ = model.Update(MsgPollEnd{Count: 5, Err: nil})
	m = updated.(*TUIModel)
	assert.Contains(t, m.PollStatus, "Poll OK (5 items)")
	assert.False(t, m.LastPoll.IsZero())

	// Test MsgSpawnStart
	item := WorkItem{ID: "TASK-1", Summary: "Do work"}
	updated, _ = model.Update(MsgSpawnStart{Item: item})
	m = updated.(*TUIModel)
	assert.Equal(t, item, m.ActiveAgents["TASK-1"])
	assert.Equal(t, "Spawning", m.AgentStatus["TASK-1"])

	// Test MsgSpawnEnd
	updated, _ = model.Update(MsgSpawnEnd{Item: item, Err: nil})
	m = updated.(*TUIModel)
	assert.Equal(t, "Running", m.AgentStatus["TASK-1"])

	// Test MsgLog
	updated, _ = model.Update(MsgLog{Content: "Log 1"})
	m = updated.(*TUIModel)
	assert.Equal(t, []string{"Log 1"}, m.Logs)

	// Test Log Rotation
	for i := 0; i < 25; i++ {
		updated, _ = model.Update(MsgLog{Content: "Log"})
	}
	m = updated.(*TUIModel)
	assert.Len(t, m.Logs, 20)
}
