package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {
	store := &MockStore{}
	s := NewServer(store, 8080, "test-project")
	if s.port != 8080 {
		t.Errorf("expected port 8080, got %d", s.port)
	}
	if s.projectID != "test-project" {
		t.Errorf("expected projectID test-project, got %s", s.projectID)
	}
	if s.store != store {
		t.Error("expected store to be set")
	}

	// Default project ID
	s2 := NewServer(store, 8080, "")
	if s2.projectID != "default" {
		t.Errorf("expected default projectID, got %s", s2.projectID)
	}
}

func TestServer_HandleFeatures(t *testing.T) {
	tests := []struct {
		name           string
		projectID      string
		featuresMap    map[string]string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:      "Valid Features",
			projectID: "test-project",
			featuresMap: map[string]string{
				"test-project": `{"project_name": "test", "features": [{"id": "1", "description": "foo"}]}`,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"id":"1"`,
		},
		{
			name:      "Fallback to Default",
			projectID: "unknown",
			featuresMap: map[string]string{
				"default": `{"project_name": "def", "features": [{"id": "2", "description": "bar"}]}`,
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `"id":"2"`,
		},
		{
			name:        "Empty Features (No Fallback)",
			projectID:   "empty",
			featuresMap: map[string]string{
				// Empty map, so neither project nor default exists
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `[]`,
		},
		{
			name:      "Invalid JSON",
			projectID: "invalid",
			featuresMap: map[string]string{
				"invalid": `{invalid_json`,
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Failed to parse features",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := &MockStore{
				GetFeaturesFunc: func(projectID string) (string, error) {
					if v, ok := tc.featuresMap[projectID]; ok {
						return v, nil
					}
					return "", errors.New("not found")
				},
			}
			s := NewServer(store, 8080, tc.projectID)
			req := httptest.NewRequest("GET", "/api/features", nil)
			w := httptest.NewRecorder()

			s.handleFeatures(w, req)

			resp := w.Result()
			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			body := w.Body.String()
			if tc.expectedBody != "" && !strings.Contains(body, tc.expectedBody) {
				t.Errorf("expected body to contain '%s', got '%s'", tc.expectedBody, body)
			}
		})
	}
}

func TestServer_HandleGraph(t *testing.T) {
	featuresMap := map[string]string{
		"test-project": `{"project_name": "test", "features": [{"id": "1", "description": "foo", "steps": ["s1"]}]}`,
		"invalid":      `{invalid_json`,
	}

	store := &MockStore{
		GetFeaturesFunc: func(projectID string) (string, error) {
			if v, ok := featuresMap[projectID]; ok {
				return v, nil
			}
			return "", errors.New("not found")
		},
	}

	tests := []struct {
		name         string
		projectID    string
		expectedBody string
	}{
		{
			name:         "Valid Graph",
			projectID:    "test-project",
			expectedBody: "graph TD",
		},
		{
			name:         "No Data",
			projectID:    "unknown",
			expectedBody: "Error[No Data Found]",
		},
		{
			name:         "Invalid JSON",
			projectID:    "invalid",
			expectedBody: "Error[Invalid Data]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(store, 8080, tc.projectID)
			req := httptest.NewRequest("GET", "/api/graph", nil)
			w := httptest.NewRecorder()

			s.handleGraph(w, req)

			body := w.Body.String()
			if !strings.Contains(body, tc.expectedBody) {
				t.Errorf("expected body to contain '%s', got '%s'", tc.expectedBody, body)
			}
		})
	}
}
