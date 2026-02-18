package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrchestrator_Verify(t *testing.T) {
	tests := []struct {
		name        string
		pollerErr   error
		spawnerErr  error
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Success",
			pollerErr:   nil,
			spawnerErr:  nil,
			expectError: false,
		},
		{
			name:        "Poller Failure",
			pollerErr:   errors.New("poller unreachable"),
			spawnerErr:  nil,
			expectError: true,
			errorMsg:    "poller check failed",
		},
		{
			name:        "Spawner Failure",
			pollerErr:   nil,
			spawnerErr:  errors.New("spawner unreachable"),
			expectError: true,
			errorMsg:    "spawner check failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			poller := newMockPoller(nil)
			poller.pingErr = tt.pollerErr

			spawner := &mockSpawner{}
			spawner.pingErr = tt.spawnerErr

			orch := New(poller, spawner, 1*time.Minute)

			err := orch.Verify(context.Background(), silentLogger)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
