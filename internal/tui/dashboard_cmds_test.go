package tui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeletePendingCmd(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		handler        http.HandlerFunc
		expectedErr    bool
		expectedErrMsg string
		expectedMsg    string
	}{
		{
			name: "success",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodDelete, r.Method)
				assert.Equal(t, "/jobs/test-id/pending", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectedMsg: "Pending job deleted",
		},
		{
			name: "error status",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedErr:    true,
			expectedErrMsg: "status 500",
		},
		{
			name:        "request error",
			url:         "http://invalid-url\x00",
			expectedErr: true,
		},
		{
			name:        "do error",
			url:         "http://localhost:0",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.url
			if tt.url == "mock" {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				url = ts.URL
			}

			cmd := deletePendingCmd(url, "test-id")
			msg := cmd()

			actionMsg, ok := msg.(actionMsg)
			assert.True(t, ok)

			if tt.expectedErr {
				assert.Error(t, actionMsg.Err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, actionMsg.Err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, actionMsg.Err)
				assert.Equal(t, tt.expectedMsg, actionMsg.Message)
			}
		})
	}
}

func TestFetchCriticalPathCmd(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		handler        http.HandlerFunc
		expectedErr    bool
		expectedErrMsg string
		expectedPath   string
	}{
		{
			name: "success",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "/jobs", r.URL.Path)
				assert.Equal(t, "all", r.URL.Query().Get("state"))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[{"id":"job1","status":"Completed","start_time":"2023-01-01T00:00:00Z","end_time":"2023-01-01T00:01:00Z"}]`))
			},
			expectedPath: "job1",
		},
		{
			name: "error status",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`Server Error`))
			},
			expectedErr:    true,
			expectedErrMsg: "status 500",
		},
		{
			name:        "bad url",
			url:         "http://[::1]:namedport",
			expectedErr: true,
		},
		{
			name:        "request error",
			url:         "http://invalid-url\x00",
			expectedErr: true,
		},
		{
			name:        "do error",
			url:         "http://localhost:0",
			expectedErr: true,
		},
		{
			name: "bad json",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`invalid json`))
			},
			expectedErr:    true,
			expectedErrMsg: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.url
			if tt.url == "mock" {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				url = ts.URL
			}

			cmd := fetchCriticalPathCmd(url)
			msg := cmd()

			cpMsg, ok := msg.(criticalPathMsg)
			assert.True(t, ok)

			if tt.expectedErr {
				assert.Error(t, cpMsg.Err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, cpMsg.Err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, cpMsg.Err)
				assert.Len(t, cpMsg.Path, 1)
				assert.Equal(t, tt.expectedPath, cpMsg.Path[0].ID)
			}
		})
	}
}

func TestPromoteJobCmd(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		handler        http.HandlerFunc
		expectedErr    bool
		expectedErrMsg string
		expectedMsg    string
	}{
		{
			name: "success",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/test-id/promote", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectedMsg: "Promoted",
		},
		{
			name: "error status",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedErr:    true,
			expectedErrMsg: "status 500",
		},
		{
			name:        "request error",
			url:         "http://invalid-url\x00",
			expectedErr: true,
		},
		{
			name:        "do error",
			url:         "http://localhost:0",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.url
			if tt.url == "mock" {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				url = ts.URL
			}

			cmd := promoteJobCmd(url, "test-id")
			msg := cmd()

			actionMsg, ok := msg.(actionMsg)
			assert.True(t, ok)

			if tt.expectedErr {
				assert.Error(t, actionMsg.Err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, actionMsg.Err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, actionMsg.Err)
				assert.Equal(t, tt.expectedMsg, actionMsg.Message)
			}
		})
	}
}

func TestDemoteJobCmd(t *testing.T) {
	tests := []struct {
		name           string
		url            string
		handler        http.HandlerFunc
		expectedErr    bool
		expectedErrMsg string
		expectedMsg    string
	}{
		{
			name: "success",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "/jobs/test-id/demote", r.URL.Path)
				w.WriteHeader(http.StatusOK)
			},
			expectedMsg: "Demoted",
		},
		{
			name: "error status",
			url:  "mock",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedErr:    true,
			expectedErrMsg: "status 500",
		},
		{
			name:        "request error",
			url:         "http://invalid-url\x00",
			expectedErr: true,
		},
		{
			name:        "do error",
			url:         "http://localhost:0",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := tt.url
			if tt.url == "mock" {
				ts := httptest.NewServer(tt.handler)
				defer ts.Close()
				url = ts.URL
			}

			cmd := demoteJobCmd(url, "test-id")
			msg := cmd()

			actionMsg, ok := msg.(actionMsg)
			assert.True(t, ok)

			if tt.expectedErr {
				assert.Error(t, actionMsg.Err)
				if tt.expectedErrMsg != "" {
					assert.Contains(t, actionMsg.Err.Error(), tt.expectedErrMsg)
				}
			} else {
				assert.NoError(t, actionMsg.Err)
				assert.Equal(t, tt.expectedMsg, actionMsg.Message)
			}
		})
	}
}
