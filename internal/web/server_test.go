package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"recac/internal/db"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockStoreWithTestify is a mock implementation of db.Store using testify/mock
type MockStoreWithTestify struct {
	mock.Mock
}

func (m *MockStoreWithTestify) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockStoreWithTestify) SaveObservation(projectID, agentID, content string) error {
	args := m.Called(projectID, agentID, content)
	return args.Error(0)
}

func (m *MockStoreWithTestify) QueryHistory(projectID string, limit int) ([]db.Observation, error) {
	args := m.Called(projectID, limit)
	return args.Get(0).([]db.Observation), args.Error(1)
}

func (m *MockStoreWithTestify) SetSignal(projectID, key, value string) error {
	args := m.Called(projectID, key, value)
	return args.Error(0)
}

func (m *MockStoreWithTestify) GetSignal(projectID, key string) (string, error) {
	args := m.Called(projectID, key)
	return args.String(0), args.Error(1)
}

func (m *MockStoreWithTestify) DeleteSignal(projectID, key string) error {
	args := m.Called(projectID, key)
	return args.Error(0)
}

func (m *MockStoreWithTestify) SaveFeatures(projectID string, features string) error {
	args := m.Called(projectID, features)
	return args.Error(0)
}

func (m *MockStoreWithTestify) GetFeatures(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}

func (m *MockStoreWithTestify) SaveSpec(projectID string, spec string) error {
	args := m.Called(projectID, spec)
	return args.Error(0)
}

func (m *MockStoreWithTestify) GetSpec(projectID string) (string, error) {
	args := m.Called(projectID)
	return args.String(0), args.Error(1)
}

func (m *MockStoreWithTestify) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	args := m.Called(projectID, id, status, passes)
	return args.Error(0)
}

func (m *MockStoreWithTestify) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) {
	args := m.Called(projectID, path, agentID, timeout)
	return args.Bool(0), args.Error(1)
}

func (m *MockStoreWithTestify) ReleaseLock(projectID, path, agentID string) error {
	args := m.Called(projectID, path, agentID)
	return args.Error(0)
}

func (m *MockStoreWithTestify) ReleaseAllLocks(projectID, agentID string) error {
	args := m.Called(projectID, agentID)
	return args.Error(0)
}

func (m *MockStoreWithTestify) GetActiveLocks(projectID string) ([]db.Lock, error) {
	args := m.Called(projectID)
	return args.Get(0).([]db.Lock), args.Error(1)
}

func (m *MockStoreWithTestify) Cleanup() error {
	args := m.Called()
	return args.Error(0)
}

func TestHandleFeatures(t *testing.T) {
	mockStore := new(MockStoreWithTestify)
	server := NewServer(mockStore, 8080, "test-project")

	features := db.FeatureList{
		Features: []db.Feature{
			{ID: "f1", Category: "Test"},
		},
	}
	featuresJSON, _ := json.Marshal(features)

	mockStore.On("GetFeatures", "test-project").Return(string(featuresJSON), nil)

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleFeatures)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response []db.Feature
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 1)
	assert.Equal(t, "f1", response[0].ID)
}

func TestHandleFeatures_Empty(t *testing.T) {
	mockStore := new(MockStoreWithTestify)
	server := NewServer(mockStore, 8080, "test-project")

	mockStore.On("GetFeatures", "test-project").Return("", nil)
	mockStore.On("GetFeatures", "default").Return("", nil)

	req, _ := http.NewRequest("GET", "/api/features", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleFeatures)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "[]", rr.Body.String())
}

func TestHandleGraph(t *testing.T) {
	mockStore := new(MockStoreWithTestify)
	server := NewServer(mockStore, 8080, "test-project")

	features := db.FeatureList{
		Features: []db.Feature{
			{ID: "f1", Category: "Test"},
			{ID: "f2", Category: "Test", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"f1"}}},
		},
	}
	featuresJSON, _ := json.Marshal(features)

	mockStore.On("GetFeatures", "test-project").Return(string(featuresJSON), nil)

	req, _ := http.NewRequest("GET", "/api/graph", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleGraph)

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	output := rr.Body.String()
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "f1")
	assert.Contains(t, output, "f2")
	assert.Contains(t, output, "f1 --> f2")
}
