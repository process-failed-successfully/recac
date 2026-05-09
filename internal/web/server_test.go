package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"recac/internal/runner"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_HandleFeatures(t *testing.T) {
	tests := []struct {
		name           string
		features       string
		storeErr       error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Success",
			features:       `{"project_name":"test","features":[{"id":"1","description":"feat 1"}]}`,
			expectedStatus: http.StatusOK,
			expectedBody:   `[{"id":"1"`,
		},
		{
			name:           "DB Error",
			storeErr:       errors.New("db fail"),
			expectedStatus: http.StatusOK, // Handler writes empty list on error
			expectedBody:   "[]",
		},
		{
			name:           "Empty Content",
			features:       "",
			expectedStatus: http.StatusOK,
			expectedBody:   "[]",
		},
		{
			name:           "Invalid JSON",
			features:       `{invalid}`,
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to parse features",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &MockStore{
				GetFeaturesFunc: func(projectID string) (string, error) {
					if tt.storeErr != nil {
						return "", tt.storeErr
					}
					return tt.features, nil
				},
			}
			server := NewServer(store, 8080, "test")

			req := httptest.NewRequest("GET", "/api/features", nil)
			w := httptest.NewRecorder()

			server.handleFeatures(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestServer_HandleGraph(t *testing.T) {
	tests := []struct {
		name           string
		features       string
		storeErr       error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "Success",
			features:       `{"project_name":"test","features":[{"id":"1","description":"feat 1","status":"done"}]}`,
			expectedStatus: http.StatusOK,
			expectedBody:   "graph TD",
		},
		{
			name:           "Success with styling failed",
			features:       `{"project_name":"test","features":[{"id":"1","description":"feat 1","status":"failed"}]}`,
			expectedStatus: http.StatusOK,
			expectedBody:   ":::failed",
		},
		{
			name:           "Success with styling in_progress",
			features:       `{"project_name":"test","features":[{"id":"1","description":"feat 1","status":"in_progress"}]}`,
			expectedStatus: http.StatusOK,
			expectedBody:   ":::inprogress",
		},
		{
			name:           "DB Error",
			storeErr:       errors.New("db fail"),
			expectedStatus: http.StatusOK,
			expectedBody:   "Error[No Data Found]",
		},
		{
			name:           "Invalid JSON",
			features:       `{invalid}`,
			expectedStatus: http.StatusOK,
			expectedBody:   "Error[Invalid Data]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &MockStore{
				GetFeaturesFunc: func(projectID string) (string, error) {
					if tt.storeErr != nil {
						return "", tt.storeErr
					}
					return tt.features, nil
				},
			}
			server := NewServer(store, 8080, "test")

			req := httptest.NewRequest("GET", "/api/graph", nil)
			w := httptest.NewRecorder()

			server.handleGraph(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
			if !strings.Contains(w.Body.String(), tt.expectedBody) {
				t.Errorf("Expected body to contain %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestServer_Start_PortInUse(t *testing.T) {
	// Start a dummy server to occupy a port
	dummyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer dummyServer.Close()

	// Extract the port from the dummy server
	port := 0
	fmt.Sscanf(dummyServer.URL, "http://127.0.0.1:%d", &port)
	if port == 0 {
		// some environments may use localhost or ipv6, let's just parse the last part
		parts := strings.Split(dummyServer.URL, ":")
		if len(parts) > 2 {
			fmt.Sscanf(parts[len(parts)-1], "%d", &port)
		}
	}

	// Ensure we got a valid port
	require.NotEqual(t, 0, port, "could not parse port from dummy server URL: %s", dummyServer.URL)

	store := &MockStore{}
	server := NewServer(store, port, "test")

	// Test the OpenBrowser path slightly, though utils.OpenBrowser will just try to run a command
	server.OpenBrowser = true

	// Start should fail because the port is already in use
	err := server.Start()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind")
}

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with-dash", "with_dash"},
		{"with space", "with_space"},
		{"with.dot", "with_dot"},
		{"complex-id.with space", "complex_id_with_space"},
	}

	for _, tt := range tests {
		if got := sanitizeMermaidID(tt.input); got != tt.expected {
			t.Errorf("sanitizeMermaidID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestServer_GenerateMermaid(t *testing.T) {
	tests := []struct {
		name     string
		nodes    map[string]*runner.TaskNode
		expected []string
	}{
		{
			name: "All Statuses",
			nodes: map[string]*runner.TaskNode{
				"1": {ID: "1", Name: "Done Task", Status: runner.TaskDone},
				"2": {ID: "2", Name: "In Progress Task", Status: runner.TaskInProgress},
				"3": {ID: "3", Name: "Failed Task", Status: runner.TaskFailed},
				"4": {ID: "4", Name: "Ready Task", Status: runner.TaskReady},
				"5": {ID: "5", Name: "Pending Task", Status: ""}, // default
			},
			expected: []string{
				"1[\"Done Task\"]:::done",
				"2[\"In Progress Task\"]:::inprogress",
				"3[\"Failed Task\"]:::failed",
				"4[\"Ready Task\"]:::ready",
				"5[\"Pending Task\"]:::pending",
			},
		},
		{
			name: "Sanitize Quotes and Newlines",
			nodes: map[string]*runner.TaskNode{
				"1": {ID: "1", Name: "Task \"with quotes\" \n and newlines"},
			},
			expected: []string{
				"1[\"Task 'with quotes'   and ne...\"]:::pending",
			},
		},
		{
			name: "Truncate Long Names",
			nodes: map[string]*runner.TaskNode{
				"1": {ID: "1", Name: "This is a very long task name that should be truncated"},
			},
			expected: []string{
				"1[\"This is a very long task na...\"]:::pending",
			},
		},
		{
			name: "With Dependencies",
			nodes: map[string]*runner.TaskNode{
				"1": {ID: "1", Name: "Task 1", Dependencies: []string{"2", "3"}},
				"2": {ID: "2", Name: "Task 2"},
				"3": {ID: "3", Name: "Task 3"},
			},
			expected: []string{
				"1[\"Task 1\"]:::pending",
				"2 --> 1",
				"3 --> 1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &runner.TaskGraph{Nodes: tt.nodes}
			mermaid := generateMermaid(g)
			for _, exp := range tt.expected {
				if !strings.Contains(mermaid, exp) {
					t.Errorf("Expected mermaid to contain %q, but got:\n%s", exp, mermaid)
				}
			}
		})
	}
}
