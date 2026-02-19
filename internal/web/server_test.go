package web

import (
	"net/http"
	"net/http/httptest"
	"recac/internal/db"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

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

func TestServer_Handler_Features(t *testing.T) {
	mockStore := new(TestifyMockStore)
	server := NewServer(mockStore, 8080, "test-project")
	handler := server.Handler()

	t.Run("Features Found", func(t *testing.T) {
		featuresJSON := `{"project_name": "test", "features": [{"id": "F1", "status": "done"}]}`
		mockStore.On("GetFeatures", "test-project").Return(featuresJSON, nil).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"id":"F1"`)
	})

	t.Run("Features Fallback Default", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("", nil).Once()
		mockStore.On("GetFeatures", "default").Return(`{"project_name": "default", "features": []}`, nil).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "[]\n", w.Body.String())
	})

	t.Run("Features Not Found", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("", nil).Once()
		mockStore.On("GetFeatures", "default").Return("", nil).Once()

		req := httptest.NewRequest("GET", "/api/features", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "[]", w.Body.String())
	})
}

func TestServer_Handler_Graph(t *testing.T) {
	mockStore := new(TestifyMockStore)
	server := NewServer(mockStore, 8080, "test-project")
	handler := server.Handler()

	t.Run("Graph Generated", func(t *testing.T) {
		featuresJSON := `{
			"project_name": "test",
			"features": [
				{"id": "Done", "status": "done"},
				{"id": "Progress", "status": "in_progress"},
				{"id": "Failed", "status": "failed"},
				{"id": "Ready", "status": "ready"},
				{"id": "Pending", "status": "pending"}
			]
		}`
		mockStore.On("GetFeatures", "test-project").Return(featuresJSON, nil).Once()

		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "graph TD")
		assert.Contains(t, body, "Done")
		assert.Contains(t, body, ":::done")
		assert.Contains(t, body, "Progress")
		// The status mapping in runner/taskgraph.go might differ from "in_progress" string in JSON?
		// internal/db/types.go says Status string.
		// generateMermaid uses runner.TaskNode.Status which is string?
		// runner/taskgraph.go defines constants.
		// TaskDone = "done", TaskInProgress = "in_progress", etc.
		// So "in_progress" should map to ":::inprogress".
		assert.Contains(t, body, ":::inprogress")
		assert.Contains(t, body, "Failed")
		assert.Contains(t, body, ":::failed")
		assert.Contains(t, body, "Ready")
		// LoadFromFeatures maps unknown statuses to TaskPending, so "ready" becomes "pending"
		assert.Contains(t, body, ":::pending")
	})

	t.Run("Graph Not Found", func(t *testing.T) {
		mockStore.On("GetFeatures", "test-project").Return("", nil).Once()
		mockStore.On("GetFeatures", "default").Return("", nil).Once()

		req := httptest.NewRequest("GET", "/api/graph", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code) // It returns 200 with error text in body
		assert.Contains(t, w.Body.String(), "Error[No Data Found]")
	})
}

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"foo bar", "foo_bar"},
		{"foo-bar", "foo_bar"},
		{"foo.bar", "foo_bar"},
		{"foo bar-baz.qux", "foo_bar_baz_qux"},
	}

	for _, tt := range tests {
		result := sanitizeMermaidID(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
