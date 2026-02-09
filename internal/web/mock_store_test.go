package web

import (
	"recac/internal/db"
	"time"
)

type MockStore struct {
	GetFeaturesFunc func(projectID string) (string, error)
}

func (m *MockStore) GetFeatures(projectID string) (string, error) {
	if m.GetFeaturesFunc != nil {
		return m.GetFeaturesFunc(projectID)
	}
	return "", nil
}

// Implement other methods to satisfy db.Store interface
func (m *MockStore) Close() error { return nil }
func (m *MockStore) SaveObservation(projectID, agentID, content string) error { return nil }
func (m *MockStore) QueryHistory(projectID string, limit int) ([]db.Observation, error) { return nil, nil }
func (m *MockStore) SetSignal(projectID, key, value string) error { return nil }
func (m *MockStore) GetSignal(projectID, key string) (string, error) { return "", nil }
func (m *MockStore) DeleteSignal(projectID, key string) error { return nil }
func (m *MockStore) SaveFeatures(projectID string, features string) error { return nil }
func (m *MockStore) SaveSpec(projectID string, spec string) error { return nil }
func (m *MockStore) GetSpec(projectID string) (string, error) { return "", nil }
func (m *MockStore) UpdateFeatureStatus(projectID, id string, status string, passes bool) error { return nil }
func (m *MockStore) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) { return true, nil }
func (m *MockStore) ReleaseLock(projectID, path, agentID string) error { return nil }
func (m *MockStore) ReleaseAllLocks(projectID, agentID string) error { return nil }
func (m *MockStore) GetActiveLocks(projectID string) ([]db.Lock, error) { return nil, nil }
func (m *MockStore) Cleanup() error { return nil }
