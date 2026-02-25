package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestDashboardCmd(t *testing.T) {
	// Mocks
	originalGet := dashboardGetFunc
	originalStart := startDashboardFunc
	defer func() {
		dashboardGetFunc = originalGet
		startDashboardFunc = originalStart
	}()

	// Setup Config
	viper.Set("orchestrator.host", "http://mock-host")

	tests := []struct {
		name        string
		mockGet     func(url string) (*http.Response, error)
		mockStart   func(host string) error
		expectError bool
		errorMsg    string
	}{
		{
			name: "Success",
			mockGet: func(url string) (*http.Response, error) {
				assert.Equal(t, "http://mock-host/status", url)
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("OK")),
				}, nil
			},
			mockStart: func(host string) error {
				assert.Equal(t, "http://mock-host", host)
				return nil
			},
			expectError: false,
		},
		{
			name: "ConnectionError",
			mockGet: func(url string) (*http.Response, error) {
				return nil, errors.New("connection refused")
			},
			mockStart:   nil, // Should not be called
			expectError: true,
			errorMsg:    "failed to connect to orchestrator",
		},
		{
			name: "OrchestratorError",
			mockGet: func(url string) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Body:       io.NopCloser(strings.NewReader("Error")),
				}, nil
			},
			mockStart:   nil,
			expectError: true,
			errorMsg:    "orchestrator returned status 500",
		},
		{
			name: "TUIError",
			mockGet: func(url string) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("OK")),
				}, nil
			},
			mockStart: func(host string) error {
				return errors.New("tui failed")
			},
			expectError: true,
			errorMsg:    "dashboard failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dashboardGetFunc = tt.mockGet
			if tt.mockStart != nil {
				startDashboardFunc = tt.mockStart
			} else {
				startDashboardFunc = func(host string) error {
					t.Error("startDashboardFunc should not be called")
					return nil
				}
			}

			// Run command
			cmd := dashboardCmd
			// Capture stdout to avoid clutter
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)

			err := cmd.RunE(cmd, []string{})
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
