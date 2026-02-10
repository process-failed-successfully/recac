package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"recac/internal/db"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestifyMockStore implements db.Store interface
type TestifyMockStore struct {
	mock.Mock
}

func (m *TestifyMockStore) Close() error {
	return m.Called().Error(0)
}

func (m *TestifyMockStore) SaveObservation(projectID, agentID, content string) error {
	return m.Called(projectID, agentID, content).Error(0)
}

func (m *TestifyMockStore) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	args := m.Called(projectID, limit)
	return args.Get(0).([]db.Observation), args.Error(1)
}

func (m *TestifyMockStore) SetSignal(projectID, key, value string) error {
	return m.Called(projectID, key, value).Error(0)
}

func (m *TestifyMockStore) GetSignal(projectID, key string) (string, error) {
	args := m.Called(projectID, key)
	return args.String(0), args.Error(1)
}

func (m *TestifyMockStore) DeleteSignal(projectID, key string) error {
	return m.Called(projectID, key).Error(0)
}

func (m *TestifyMockStore) SaveFeatures(projectID string, features string) error {
	return m.Called(projectID, features).Error(0)
}

func (m *TestifyMockStore) GetFeatures(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}

func (m *TestifyMockStore) SaveSpec(projectID string, spec string) error {
	return m.Called(projectID, spec).Error(0)
}

func (m *TestifyMockStore) GetSpec(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}

func (m *TestifyMockStore) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	return m.Called(projectID, id, status, passes).Error(0)
}

func (m *TestifyMockStore) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	args := m.Called(projectID, path, agentID, timeout)
	return args.Bool(0), args.Error(1)
}

func (m *TestifyMockStore) ReleaseLock(projectID, path, agentID string) error {
	return m.Called(projectID, path, agentID).Error(0)
}

func (m *TestifyMockStore) ReleaseAllLocks(projectID, agentID string) error {
	return m.Called(projectID, agentID).Error(0)
}

func (m *TestifyMockStore) GetActiveLocks(projectID string) ([]db.Lock, error) {
	args := m.Called(projectID)
	return args.Get(0).([]db.Lock), args.Error(1)
}

func (m *TestifyMockStore) Cleanup() error {
	return m.Called().Error(0)
}

func TestNewServer(t *testing.T) {
	store := new(TestifyMockStore)
	server := NewServer(store, 8080, "test-project")
	assert.NotNil(t, server)
	assert.Equal(t, store, server.store)
	assert.Equal(t, 8080, server.port)
	assert.Equal(t, "test-project", server.projectID)

	// Default project
	serverDefault := NewServer(store, 8080, "")
	assert.Equal(t, "default", serverDefault.projectID)
}

func TestHandleFeatures(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		featureList := db.FeatureList{
			ProjectName: "test-project",
			Features: []db.Feature{
				{ID: "F1", Description: "Feature 1"},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)
		store.On("GetFeatures", "test-project").Return(string(jsonBytes), nil).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()

		server.handleFeatures(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var features []db.Feature
		err := json.NewDecoder(resp.Body).Decode(&features)
		assert.NoError(t, err)
		assert.Len(t, features, 1)
		assert.Equal(t, "F1", features[0].ID)
	})

	t.Run("fallback to default", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		store.On("GetFeatures", "test-project").Return("", nil).Once()

		featureList := db.FeatureList{
			ProjectName: "default",
			Features: []db.Feature{
				{ID: "F2", Description: "Feature 2"},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)
		store.On("GetFeatures", "default").Return(string(jsonBytes), nil).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()

		server.handleFeatures(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var features []db.Feature
		err := json.NewDecoder(resp.Body).Decode(&features)
		assert.NoError(t, err)
		assert.Len(t, features, 1)
		assert.Equal(t, "F2", features[0].ID)
	})

	t.Run("not found", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		store.On("GetFeatures", "test-project").Return("", errors.New("not found")).Once()
		store.On("GetFeatures", "default").Return("", errors.New("not found")).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()

		server.handleFeatures(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode) // Returns empty list with 200 OK as per implementation

		body := w.Body.String()
		assert.Equal(t, "[]", body)
	})

	t.Run("invalid json", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		store.On("GetFeatures", "test-project").Return("invalid-json", nil).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()

		server.handleFeatures(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	})
}

func TestHandleGraph(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		featureList := db.FeatureList{
			ProjectName: "test-project",
			Features: []db.Feature{
				{ID: "F1", Description: "Feature 1", Status: "done"},
				{ID: "F2", Description: "Feature 2", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"F1"}}},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)
		store.On("GetFeatures", "test-project").Return(string(jsonBytes), nil).Once()

		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()

		server.handleGraph(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body := w.Body.String()
		assert.Contains(t, body, "graph TD")
		assert.Contains(t, body, "F1")
		assert.Contains(t, body, "F2")
		assert.Contains(t, body, "F1 --> F2") // Order might vary but F1 --> F2 implies dependency logic if implemented correctly
		// Wait, F2 depends on F1. So F1 is prerequisite. Graph usually draws dependency direction.
		// recac/internal/runner/graph.go usually implements "depends on" as edges.
		// generateMermaid: "safeDepID --> safeID" which means "F1 --> F2" if F2 depends on F1.
	})

	t.Run("no data", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		store.On("GetFeatures", "test-project").Return("", nil).Once()
		store.On("GetFeatures", "default").Return("", nil).Once()

		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()

		server.handleGraph(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, w.Body.String(), "Error[No Data Found]")
	})

	t.Run("invalid data", func(t *testing.T) {
		store := new(TestifyMockStore)
		server := NewServer(store, 8080, "test-project")
		store.On("GetFeatures", "test-project").Return("invalid", nil).Once()

		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()

		server.handleGraph(w, req)

		resp := w.Result()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, w.Body.String(), "Error[Invalid Data]")
	})
}

func TestSanitizeMermaidID(t *testing.T) {
	assert.Equal(t, "foo_bar", sanitizeMermaidID("foo bar"))
	assert.Equal(t, "foo_bar", sanitizeMermaidID("foo-bar"))
	assert.Equal(t, "foo_bar", sanitizeMermaidID("foo.bar"))
}
