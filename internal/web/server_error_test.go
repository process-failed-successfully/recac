package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_Handlers_Error(t *testing.T) {
	mockStore := &MockStore{}
	s := NewServer(mockStore, 0, "test-project")

	// Case 1: GetFeatures Error
	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "", errors.New("db error")
	}

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	s.handleFeatures(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]", w.Body.String()) // Fallback to empty list

	// Case 2: Invalid JSON
	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "{invalid-json", nil
	}

	req = httptest.NewRequest("GET", "/api/features", nil)
	w = httptest.NewRecorder()
	s.handleFeatures(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "Failed to parse features")

	// Case 3: Graph Error (DB Error)
	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "", errors.New("db error")
	}
	req = httptest.NewRequest("GET", "/api/graph", nil)
	w = httptest.NewRecorder()
	s.handleGraph(w, req)
	assert.Equal(t, http.StatusOK, w.Code) // Writes error to body
	assert.Contains(t, w.Body.String(), "Error[No Data Found]")

	// Case 4: Graph Error (Invalid JSON)
	mockStore.GetFeaturesFunc = func(projectID string) (string, error) {
		return "{invalid", nil
	}
	req = httptest.NewRequest("GET", "/api/graph", nil)
	w = httptest.NewRecorder()
	s.handleGraph(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Error[Invalid Data]")
}
