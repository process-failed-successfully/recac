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

	s := NewServer(store, 0)

	// Test /api/features
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

	// Test /api/graph
	req = httptest.NewRequest("GET", "/api/graph", nil)
	w = httptest.NewRecorder()
	s.handleGraph(w, req)

	resp = w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// We expect Mermaid graph definition
	// graph TD
	// A["Task A"]:::done
	// B["Task B"]:::pending
	// A --> B
	// ... (plus class defs)

	body := w.Body.String()
	assert.Contains(t, body, "graph TD")
	assert.Contains(t, body, "A[\"Task A\"]:::done")
	assert.Contains(t, body, "B[\"Task B\"]:::pending")
	assert.Contains(t, body, "A --> B")
}
