package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStoreWithTestify implements db.Store interface
type MockStoreWithTestify struct {
	mock.Mock
}

func (m *MockStoreWithTestify) Close() error { return m.Called().Error(0) }
func (m *MockStoreWithTestify) SaveObservation(projectID, agentID, content string) error {
	return m.Called(projectID, agentID, content).Error(0)
}
func (m *MockStoreWithTestify) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	args := m.Called(projectID, limit)
	return args.Get(0).([]db.Observation), args.Error(1)
}
func (m *MockStoreWithTestify) SetSignal(projectID, key, value string) error {
	return m.Called(projectID, key, value).Error(0)
}
func (m *MockStoreWithTestify) GetSignal(projectID, key string) (string, error) {
	args := m.Called(projectID, key)
	return args.String(0), args.Error(1)
}
func (m *MockStoreWithTestify) DeleteSignal(projectID, key string) error {
	return m.Called(projectID, key).Error(0)
}
func (m *MockStoreWithTestify) SaveFeatures(projectID string, features string) error {
	return m.Called(projectID, features).Error(0)
}
func (m *MockStoreWithTestify) GetFeatures(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}
func (m *MockStoreWithTestify) SaveSpec(projectID string, spec string) error {
	return m.Called(projectID, spec).Error(0)
}
func (m *MockStoreWithTestify) GetSpec(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}
func (m *MockStoreWithTestify) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	return m.Called(projectID, id, status, passes).Error(0)
}
func (m *MockStoreWithTestify) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	args := m.Called(projectID, path, agentID, timeout)
	return args.Bool(0), args.Error(1)
}
func (m *MockStoreWithTestify) ReleaseLock(projectID, path, agentID string) error {
	return m.Called(projectID, path, agentID).Error(0)
}
func (m *MockStoreWithTestify) ReleaseAllLocks(projectID, agentID string) error {
	return m.Called(projectID, agentID).Error(0)
}
func (m *MockStoreWithTestify) GetActiveLocks(projectID string) ([]db.Lock, error) {
	args := m.Called(projectID)
	return args.Get(0).([]db.Lock), args.Error(1)
}
func (m *MockStoreWithTestify) Cleanup() error {
	return m.Called().Error(0)
}

func TestNewServer(t *testing.T) {
	mockStore := new(MockStoreWithTestify)
	t.Run("With Project", func(t *testing.T) {
		server := NewServer(mockStore, 8080, "test-project")
		assert.NotNil(t, server)
	})

	t.Run("Default Project", func(t *testing.T) {
		server := NewServer(mockStore, 8080, "")
		assert.NotNil(t, server)
		// Indirectly verify by checking if handleFeatures defaults to "default" without fallback needed?
		// Hard to verify private field 'projectID' without reflection or getter.
		// But coverage will show if the line was hit.
	})
}

func TestHandleFeatures(t *testing.T) {
	mockStore := new(MockStoreWithTestify)
	server := NewServer(mockStore, 8080, "test-project")

	t.Run("Success", func(t *testing.T) {
		featureList := db.FeatureList{
			ProjectName: "test-project",
			Features: []db.Feature{
				{ID: "F1", Description: "Feature 1"},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)

		mockStore.On("GetFeatures", "test-project").Return(string(jsonBytes), nil).Once()

		req, _ := http.NewRequest("GET", "/api/features", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(server.handleFeatures) // Method value
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Feature 1")
	})

	t.Run("Fallback to Default", func(t *testing.T) {
		featureList := db.FeatureList{
			ProjectName: "default",
			Features: []db.Feature{
				{ID: "F1", Description: "Default Feature"},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)

		// First call returns empty
		mockStore.On("GetFeatures", "test-project").Return("", nil).Once()
		// Second call (fallback) returns data
		mockStore.On("GetFeatures", "default").Return(string(jsonBytes), nil).Once()

		req, _ := http.NewRequest("GET", "/api/features", nil)
		rr := httptest.NewRecorder()

		// Accessing private method via export trick or just testing via public API if possible.
		// Since handleFeatures is private, we can't call it directly from another package if 'web' was different.
		// But we are in 'package web', so we can access 'server.handleFeatures'.
		// However, I defined 'package web' in this file.

		handler := http.HandlerFunc(server.handleFeatures)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "Default Feature")
	})

	t.Run("Store Error", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("", errors.New("db error")).Once()
		mockStore.On("GetFeatures", "default").Return("", errors.New("db error")).Once()

		req, _ := http.NewRequest("GET", "/api/features", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(server.handleFeatures)
		handler.ServeHTTP(rr, req)

		// The implementation writes "[]" on error
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "[]", rr.Body.String())
	})

	t.Run("Both Empty", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("", nil).Once()
		mockStore.On("GetFeatures", "default").Return("", nil).Once()

		req, _ := http.NewRequest("GET", "/api/features", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(server.handleFeatures)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "[]", rr.Body.String())
	})
}

func TestHandleGraph(t *testing.T) {
	mockStore := new(MockStoreWithTestify)
	server := NewServer(mockStore, 8080, "test-project")

	t.Run("Success", func(t *testing.T) {
		featureList := db.FeatureList{
			ProjectName: "test-project",
			Features: []db.Feature{
				{ID: "F1", Description: "Feature 1", Status: "done"},
				{ID: "F2", Description: "Feature 2", Status: "pending", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"F1"}}},
				{ID: "F3", Description: "Feature 3", Status: "in_progress"},
				{ID: "F4", Description: "Feature 4", Status: "failed"},
				{ID: "F5", Description: "Feature 5", Status: "ready"},
			},
		}
		jsonBytes, _ := json.Marshal(featureList)

		mockStore.On("GetFeatures", "test-project").Return(string(jsonBytes), nil).Once()

		req, _ := http.NewRequest("GET", "/api/graph", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(server.handleGraph)
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		body := rr.Body.String()
		assert.Contains(t, body, "graph TD")
		assert.Contains(t, body, "F1")
		assert.Contains(t, body, "F2")
		assert.Contains(t, body, "F1 --> F2") // Check dependency
		assert.Contains(t, body, ":::done")
	})

	t.Run("No Data", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("", nil).Once()
		mockStore.On("GetFeatures", "default").Return("", nil).Once()

		req, _ := http.NewRequest("GET", "/api/graph", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(server.handleGraph)
		handler.ServeHTTP(rr, req)

		assert.Contains(t, rr.Body.String(), "Error[No Data Found]")
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("{invalid-json", nil).Once()

		req, _ := http.NewRequest("GET", "/api/graph", nil)
		rr := httptest.NewRecorder()

		handler := http.HandlerFunc(server.handleGraph)
		handler.ServeHTTP(rr, req)

		assert.Contains(t, rr.Body.String(), "Error[Invalid Data]")
	})
}

func TestSanitizeMermaidID(t *testing.T) {
	input := "foo.bar-baz space"
	expected := "foo_bar_baz_space"
	result := sanitizeMermaidID(input)
	assert.Equal(t, expected, result)
}
