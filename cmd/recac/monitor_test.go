package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonitorCmd_Run(t *testing.T) {
	// Backup original function
	originalStartDashboard := startDashboard
	defer func() { startDashboard = originalStartDashboard }()

	tests := []struct {
		name        string
		args        []string
		expectedHost string
		mockError   error
		wantErr     bool
	}{
		{
			name:         "Default host",
			args:         []string{"monitor"},
			expectedHost: "http://localhost:2112",
			mockError:    nil,
			wantErr:      false,
		},
		{
			name:         "Custom host flag",
			args:         []string{"monitor", "--host", "http://example.com:8080"},
			expectedHost: "http://example.com:8080",
			mockError:    nil,
			wantErr:      false,
		},
		{
			name:         "Dashboard error",
			args:         []string{"monitor"},
			expectedHost: "http://localhost:2112",
			mockError:    fmt.Errorf("tui failure"),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock startDashboard
			called := false
			startDashboard = func(host string) error {
				called = true
				assert.Equal(t, tt.expectedHost, host)
				return tt.mockError
			}

			_, err := executeCommand(rootCmd, tt.args...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.True(t, called, "startDashboard should be called")
		})
	}
}
