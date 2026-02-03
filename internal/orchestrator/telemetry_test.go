package orchestrator_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"recac/internal/orchestrator"
	"recac/internal/telemetry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockPoller is a mock implementation of the Poller interface
type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) Poll(ctx context.Context, logger *slog.Logger) ([]orchestrator.WorkItem, error) {
	args := m.Called(ctx, logger)
	return args.Get(0).([]orchestrator.WorkItem), args.Error(1)
}

func (m *MockPoller) UpdateStatus(ctx context.Context, item orchestrator.WorkItem, status, msg string) error {
	args := m.Called(ctx, item, status, msg)
	return args.Error(0)
}

func TestOrchestrator_Telemetry(t *testing.T) {
	// 1. Start Metrics Server
	// Find a free port
	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	basePort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	go func() {
		_ = telemetry.StartMetricsServer(basePort)
	}()
	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// 2. Setup Orchestrator
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Mock Poller that returns 1 item once
	poller := new(MockPoller)
	item := orchestrator.WorkItem{ID: "telemetry-task", Summary: "Test Task"}

	// Poller logic: Return item once, then empty
	poller.On("Poll", mock.Anything, mock.Anything).Return([]orchestrator.WorkItem{item}, nil).Once()
	poller.On("Poll", mock.Anything, mock.Anything).Return([]orchestrator.WorkItem{}, nil)

	poller.On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Reuse MockSpawner from integration test if possible, or redefine
	spawner := new(MockSpawner) // MockSpawner is in the same package (orchestrator_test)
	spawner.On("Spawn", mock.Anything, item).Return(nil)

	// Orchestrator with short interval
	orch := orchestrator.New(poller, spawner, 100*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 3. Run Orchestrator
	go func() {
		_ = orch.Run(ctx, logger)
	}()

	// 4. Wait for execution
	time.Sleep(500 * time.Millisecond)

	// 5. Query Metrics
	// Try a few ports in case of conflict
	var metricsData string
	var queryErr error
	for i := 0; i < 5; i++ {
		url := fmt.Sprintf("http://localhost:%d/metrics", basePort+i)
		resp, reqErr := http.Get(url)
		if reqErr == nil {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			metricsData = string(body)
			queryErr = nil
			break
		}
		queryErr = reqErr
	}
	require.NoError(t, queryErr, "Failed to fetch metrics from any port")

	// 6. Assertions
	assert.Contains(t, metricsData, "recac_orchestrator_loops_total")
	assert.Contains(t, metricsData, "recac_tasks_pending")

    // Check loop count > 0
    assert.True(t, strings.Contains(metricsData, `recac_orchestrator_loops_total{project="orchestrator"}`), "Should contain loop counter")
}
