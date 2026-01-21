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

func TestNewServer(t *testing.T) {
	s := NewServer(nil, 8080, "")
	assert.Equal(t, "default", s.projectID)

	s2 := NewServer(nil, 8080, "myproject")
	assert.Equal(t, "myproject", s2.projectID)
}

func TestServer_Handlers(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// Seed data
	featuresJSON := `{
		"project_name": "test-project",
		"features": [
			{"id": "A", "description": "Task A", "status": "done", "priority": "P0", "dependencies": {}},
			{"id": "B", "description": "Task B", "status": "pending", "priority": "P1", "dependencies": {"depends_on_ids": ["A"]}}
		]
	}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	s := NewServer(store, 0, "default")

	t.Run("HandleFeatures Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		s.handleFeatures(w, req)

		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var features []db.Feature
		err = json.NewDecoder(resp.Body).Decode(&features)
		require.NoError(t, err)
		assert.Len(t, features, 2)
		assert.Equal(t, "A", features[0].ID)
		assert.Equal(t, "B", features[1].ID)
	})

	t.Run("HandleGraph Success", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		s.handleGraph(w, req)

		resp := w.Result()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body := w.Body.String()
		assert.Contains(t, body, "graph TD")
		assert.Contains(t, body, "A[\"Task A\"]:::done")
		assert.Contains(t, body, "B[\"Task B\"]:::pending")
		assert.Contains(t, body, "A --> B")
	})
}

func TestServer_Handlers_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	// 1. Missing Project (Fallback)
	// We want to request "myproject", but it's empty. "default" has data.
	featuresJSON := `{
		"project_name": "default-project",
		"features": [
			{"id": "D", "description": "Default Task", "status": "done"}
		]
	}`
	require.NoError(t, store.SaveFeatures("default", featuresJSON))

	s := NewServer(store, 0, "myproject")

	t.Run("HandleFeatures Fallback", func(t *testing.T) {
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
	})

	t.Run("HandleGraph Fallback", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		s.handleGraph(w, req)
		body := w.Body.String()
		assert.Contains(t, body, "D[\"Default Task\"]")
	})

	// 2. Empty Data (No default either)
	// Create a new store/server for this
	dbPath2 := filepath.Join(tmpDir, ".recac2.db")
	store2, _ := db.NewSQLiteStore(dbPath2)
	defer store2.Close()
	s2 := NewServer(store2, 0, "empty")

	t.Run("HandleFeatures Empty", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		s2.handleFeatures(w, req)

		// Expect empty JSON array
		body := w.Body.String()
		assert.Equal(t, "[]", body)
	})

	t.Run("HandleGraph Empty", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		s2.handleGraph(w, req)

		body := w.Body.String()
		assert.Contains(t, body, "Error[No Data Found]")
	})

	// 3. Invalid JSON in DB
	require.NoError(t, store2.SaveFeatures("broken", "{ invalid json "))
	s3 := NewServer(store2, 0, "broken")

	t.Run("HandleFeatures Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		s3.handleFeatures(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("HandleGraph Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		s3.handleGraph(w, req)

		body := w.Body.String()
		assert.Contains(t, body, "Error[Invalid Data]")
	})

	// 4. Long Name
	t.Run("HandleGraph Long Name", func(t *testing.T) {
		longName := "This is a very long task name that should be truncated because it is too long"
		featuresJSON := `{
			"features": [
				{"id": "L", "description": "` + longName + `", "status": "pending"}
			]
		}`
		require.NoError(t, store.SaveFeatures("long", featuresJSON))
		sLong := NewServer(store, 0, "long")

		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		sLong.handleGraph(w, req)

		body := w.Body.String()
		assert.Contains(t, body, "This is a very long task na...")
	})
}
