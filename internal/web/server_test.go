package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_HandleFeatures_Success(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-project")

	featuresJSON := `{"project_name":"test-project","features":[{"id":"F1","category":"Core","priority":"MVP"}]}`

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		if projectID == "test-project" {
			return featuresJSON, nil
		}
		return "", nil
	}

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	server.handleFeatures(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// Check body
	body := w.Body.String()
	assert.Contains(t, body, "F1")
	assert.Contains(t, body, "MVP")
}

func TestServer_HandleFeatures_FallbackDefault(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-project")

	featuresJSON := `{"project_name":"default","features":[{"id":"D1"}]}`

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		if projectID == "test-project" {
			return "", nil
		}
		if projectID == "default" {
			return featuresJSON, nil
		}
		return "", nil
	}

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	server.handleFeatures(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := w.Body.String()
	assert.Contains(t, body, "D1")
}

func TestServer_HandleFeatures_NotFound(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-project")

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "", errors.New("not found")
	}

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	server.handleFeatures(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode) // Should return empty list 200 OK
	assert.Equal(t, "[]", w.Body.String())
}

func TestServer_HandleFeatures_InvalidJSON(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-project")

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "invalid json", nil
	}

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()

	server.handleFeatures(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestServer_HandleGraph_Success(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-project")

	// Task graph relies on features list
	featuresJSON := `{"project_name":"test-project","features":[
		{"id":"T1","category":"Core","description":"Task 1","status":"done","dependencies":{"depends_on_ids":[]}},
		{"id":"T2","category":"Core","description":"Task 2","status":"pending","dependencies":{"depends_on_ids":["T1"]}}
	]}`

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return featuresJSON, nil
	}

	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()

	server.handleGraph(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "graph TD")
	assert.Contains(t, body, "T1")
	assert.Contains(t, body, "T2")
	assert.Contains(t, body, "T1 --> T2")
	assert.Contains(t, body, ":::done") // T1 completed
	assert.Contains(t, body, ":::pending") // T2 pending
}

func TestServer_HandleGraph_NoData(t *testing.T) {
	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test-project")

	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "", nil
	}

	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()

	server.handleGraph(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Error[No Data Found]")
}

func TestSanitizeMermaidID(t *testing.T) {
	id := "test-id.with spaces"
	sanitized := sanitizeMermaidID(id)
	assert.Equal(t, "test_id_with_spaces", sanitized)
}
