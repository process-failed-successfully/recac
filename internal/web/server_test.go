package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
