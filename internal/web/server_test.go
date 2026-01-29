package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/db"
	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStoreTestify is a mock implementation of db.Store
type MockStoreTestify struct {
	mock.Mock
}

func (m *MockStoreTestify) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStoreTestify) SaveObservation(projectID, agentID, content string) error {
	args := m.Called(projectID, agentID, content)
	return args.Error(0)
}

func (m *MockStoreTestify) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	args := m.Called(projectID, limit)
	return args.Get(0).([]db.Observation), args.Error(1)
}

func (m *MockStoreTestify) SetSignal(projectID, key, value string) error {
	args := m.Called(projectID, key, value)
	return args.Error(0)
}

func (m *MockStoreTestify) GetSignal(projectID, key string) (string, error) {
	args := m.Called(projectID, key)
	return args.String(0), args.Error(1)
}

func (m *MockStoreTestify) DeleteSignal(projectID, key string) error {
	args := m.Called(projectID, key)
	return args.Error(0)
}

func (m *MockStoreTestify) SaveFeatures(projectID string, features string) error {
	args := m.Called(projectID, features)
	return args.Error(0)
}

func (m *MockStoreTestify) GetFeatures(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}

func (m *MockStoreTestify) SaveSpec(projectID string, spec string) error {
	args := m.Called(projectID, spec)
	return args.Error(0)
}

func (m *MockStoreTestify) GetSpec(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}

func (m *MockStoreTestify) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	args := m.Called(projectID, id, status, passes)
	return args.Error(0)
}

func (m *MockStoreTestify) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	args := m.Called(projectID, path, agentID, timeout)
	return args.Bool(0), args.Error(1)
}

func (m *MockStoreTestify) ReleaseLock(projectID, path, agentID string) error {
	args := m.Called(projectID, path, agentID)
	return args.Error(0)
}

func (m *MockStoreTestify) ReleaseAllLocks(projectID, agentID string) error {
	args := m.Called(projectID, agentID)
	return args.Error(0)
}

func (m *MockStoreTestify) GetActiveLocks(projectID string) ([]db.Lock, error) {
	args := m.Called(projectID)
	return args.Get(0).([]db.Lock), args.Error(1)
}

func (m *MockStoreTestify) Cleanup() error {
	args := m.Called()
	return args.Error(0)
}

func TestNewServer(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "test-project")

	assert.NotNil(t, server)
	assert.Equal(t, 8080, server.port)
	assert.Equal(t, "test-project", server.projectID)
	assert.Equal(t, mockStore, server.store)
}

func TestNewServer_DefaultProject(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "")

	assert.Equal(t, "default", server.projectID)
}

func TestHandleFeatures(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "test-project")

	features := db.FeatureList{
		ProjectName: "test-project",
		Features: []db.Feature{
			{ID: "1", Description: "Feature 1"},
		},
	}
	featuresJSON, _ := json.Marshal(features)

	mockStore.On("GetFeatures", "test-project").Return(string(featuresJSON), nil)

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(server.handleFeatures)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var responseFeatures []db.Feature
	err := json.Unmarshal(rr.Body.Bytes(), &responseFeatures)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(responseFeatures))
	assert.Equal(t, "Feature 1", responseFeatures[0].Description)
}

func TestHandleFeatures_Fallback(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "other-project")

	mockStore.On("GetFeatures", "other-project").Return("", errors.New("not found"))
	mockStore.On("GetFeatures", "default").Return(`{"features": [{"id": "2", "description": "Default Feature"}]}`, nil)

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(server.handleFeatures)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Default Feature")
}

func TestHandleFeatures_Empty(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "default")

	mockStore.On("GetFeatures", "default").Return("", nil)

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(server.handleFeatures)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "[]", rr.Body.String())
}

func TestHandleFeatures_InvalidJSON(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "default")

	mockStore.On("GetFeatures", "default").Return("invalid json", nil)

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(server.handleFeatures)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHandleGraph(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "test-project")

	features := db.FeatureList{
		Features: []db.Feature{
			{ID: "1", Description: "Root", Status: "done"},
			{ID: "2", Description: "Child", Status: "pending", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"1"}}},
		},
	}
	featuresJSON, _ := json.Marshal(features)

	mockStore.On("GetFeatures", "test-project").Return(string(featuresJSON), nil)

	req, _ := http.NewRequest("GET", "/api/graph", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(server.handleGraph)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "graph TD")
	assert.Contains(t, rr.Body.String(), "1[\"Root\"]:::done")
	assert.Contains(t, rr.Body.String(), "2[\"Child\"]:::pending")
	assert.Contains(t, rr.Body.String(), "1 --> 2") // Dependancy direction might be other way around in mermaid syntax?
	// In graph.go: safeDepID --> safeID
	// Dependencies are "DependsOnIDs". So Child depends on Root. Root --> Child.
	// So 1 --> 2.
}

func TestHandleGraph_Error(t *testing.T) {
	mockStore := new(MockStoreTestify)
	server := NewServer(mockStore, 8080, "test-project")

	mockStore.On("GetFeatures", "test-project").Return("", errors.New("db error"))
	mockStore.On("GetFeatures", "default").Return("", errors.New("db error"))

	req, _ := http.NewRequest("GET", "/api/graph", nil)
	rr := httptest.NewRecorder()

	handler := http.HandlerFunc(server.handleGraph)
	handler.ServeHTTP(rr, req)

	assert.Contains(t, rr.Body.String(), "Error[No Data Found]")
}

func TestGenerateMermaid(t *testing.T) {
	g := runner.NewTaskGraph()
	g.AddNode("A", "Task A", nil)
	g.AddNode("B", "Task B", []string{"A"})

	g.MarkTaskStatus("A", runner.TaskDone, nil)
	g.MarkTaskStatus("B", runner.TaskInProgress, nil)

	output := generateMermaid(g)

	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "A[\"Task A\"]:::done")
	assert.Contains(t, output, "B[\"Task B\"]:::inprogress")
	assert.Contains(t, output, "A --> B")
}

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with-dash", "with_dash"},
		{"with space", "with_space"},
		{"with.dot", "with_dot"},
		{"complex- .id", "complex___id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeMermaidID(tt.input))
		})
	}
}
