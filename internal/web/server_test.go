package web

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"recac/internal/db"
	"strconv"
	"testing"
	"time"

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

	s := NewServer(store, 0, "default")

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
func TestNewServer_Defaults(t *testing.T) {
	s := NewServer(nil, 8080, "")
	assert.Equal(t, "default", s.projectID)
}

func TestHandleFeatures_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, err := db.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	s := NewServer(store, 0, "non-existent")

	req := httptest.NewRequest("GET", "/api/features", nil)
	w := httptest.NewRecorder()
	s.handleFeatures(w, req)

	resp := w.Result()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body := w.Body.String()
	assert.Equal(t, "[]", body)
}

func TestServer_Start(t *testing.T) {
	// 1. Get a free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// 2. Initialize Server with Store
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".recac.db")
	store, _ := db.NewSQLiteStore(dbPath)
	defer store.Close()

	s := NewServer(store, port, "default")

	// 3. Start in goroutine
	done := make(chan error)
	go func() {
		done <- s.Start()
	}()

	// 4. Wait for it to be ready
	// We can retry connecting until successful or timeout
	ready := false
	for i := 0; i < 20; i++ {
		resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/api/features")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if !ready {
		t.Fatal("Server failed to start")
	}

	// 5. Stop
	err = s.Stop(context.Background())
	require.NoError(t, err)

	// 6. Ensure Start returns nil
	err = <-done
	require.NoError(t, err)
}
