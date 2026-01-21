package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"recac/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (db.Store, string) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	return store, tmpDir
}

func TestServer_Features_Success(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	featuresJSON := `{
		"project_name": "test-project",
		"features": [
			{"id": "A", "description": "Task A", "status": "done"},
			{"id": "B", "description": "Task B", "status": "pending"}
		]
	}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	s.handleFeatures(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var features []db.Feature
	err := json.NewDecoder(resp.Body).Decode(&features)
	require.NoError(t, err)
	assert.Len(t, features, 2)
}

func TestServer_Features_Fallback(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	featuresJSON := `{"project_name": "default-p", "features": [{"id": "D", "description": "Default"}]}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	// Requesting project "other" which has no data, should fallback to "default"
	s := NewServer(store, 0, "other")
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	s.handleFeatures(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var features []db.Feature
	err := json.NewDecoder(resp.Body).Decode(&features)
	require.NoError(t, err)
	assert.Len(t, features, 1)
	assert.Equal(t, "D", features[0].ID)
}

func TestServer_Features_Empty(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	// No data in store
	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	s.handleFeatures(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Should return empty list
	body := w.Body.String()
	assert.Equal(t, "[]", body)
}

func TestServer_Features_InvalidJSON(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	// Save invalid JSON
	require.NoError(t, store.SaveFeatures("default", "{invalid-json"))

	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	s.handleFeatures(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestServer_Graph_Success(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	featuresJSON := `{
		"project_name": "test-project",
		"features": [
			{"id": "A", "description": "Task A", "status": "done", "priority": "P0", "dependencies": {}},
			{"id": "B", "description": "Task B", "status": "pending", "priority": "P1", "dependencies": {"depends_on_ids": ["A"]}}
		]
	}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()
	s.handleGraph(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := w.Body.String()
	assert.Contains(t, body, "graph TD")
	assert.Contains(t, body, "A[\"Task A\"]:::done")
	assert.Contains(t, body, "A --> B")
}

func TestServer_Graph_Sanitization(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	// IDs with special chars
	featuresJSON := `{
		"project_name": "test-project",
		"features": [
			{"id": "A.1-2 3", "description": "Weird \"Name\"", "status": "ready"}
		]
	}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()
	s.handleGraph(w, req)

	body := w.Body.String()
	// "A.1-2 3" -> "A_1_2_3" (assuming replaceAll logic)
	// "Weird "Name"" -> "Weird 'Name'"
	assert.Contains(t, body, "A_1_2_3")
	assert.Contains(t, body, "Weird 'Name'")
}

func TestServer_Graph_NoData(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()
	s.handleGraph(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Error[No Data Found]")
}

func TestServer_Graph_InvalidJSON(t *testing.T) {
	store, _ := setupTestStore(t)
	defer store.Close()

	require.NoError(t, store.SaveFeatures("default", "{invalid"))

	s := NewServer(store, 0, "default")
	req := httptest.NewRequest("GET", "/api/graph", nil)
	w := httptest.NewRecorder()
	s.handleGraph(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Error[Invalid Data]")
}
