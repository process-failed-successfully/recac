package orchestrator

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOrchestrator_BudgetLimit(t *testing.T) {
	poller := newMockPoller([]WorkItem{})
	spawner := &mockSpawner{}
	orch := New(poller, spawner, 100*time.Millisecond)

	orch.MaxBudget = 10.0 // Set max budget

	// Add an active job with cost that exceeds the budget
	orch.activeJobs = map[string]JobInfo{
		"job-1": {
			ID: "job-1",
			Metrics: map[string]float64{
				"cost": 15.0,
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	logger := slog.Default()

	// Wait for Run to process the polling loop at least once
	go orch.Run(ctx, logger)
	time.Sleep(150 * time.Millisecond)

	status := orch.GetStatus()
	assert.Equal(t, 15.0, status.TotalCost)
	assert.True(t, status.Paused, "Orchestrator should be paused when total cost exceeds budget")
}
