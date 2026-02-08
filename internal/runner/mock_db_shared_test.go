package runner

import (
	"encoding/json"
	"fmt"
	"recac/internal/db"
	"sync"
	"time"
)

// FaultToleranceMockDB is a shared mock implementation of db.Store for testing
type FaultToleranceMockDB struct {
	Signals      map[string]string
	Observations map[string][]db.Observation
	Specs        map[string]string
	Features     map[string]string
	FeatureList  db.FeatureList // Kept for compatibility with orchestrator tests
	mu           sync.Mutex
}

func NewFaultToleranceMockDB() *FaultToleranceMockDB {
	return &FaultToleranceMockDB{
		Signals:      make(map[string]string),
		Observations: make(map[string][]db.Observation),
		Specs:        make(map[string]string),
		Features:     make(map[string]string),
	}
}

func (m *FaultToleranceMockDB) SetSignal(project, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Use both key styles to be safe
	m.Signals[project+"_"+key] = value // Standard
	m.Signals[key] = value             // Orchestrator style (often just key)
	return nil
}

func (m *FaultToleranceMockDB) GetSignal(project, key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if val, ok := m.Signals[project+"_"+key]; ok {
		return val, nil
	}
	if val, ok := m.Signals[key]; ok {
		return val, nil
	}
	return "", nil // Return empty if not found, standard behavior for mocks often
}

func (m *FaultToleranceMockDB) DeleteSignal(project, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Signals, project+"_"+key)
	delete(m.Signals, key)
	return nil
}

func (m *FaultToleranceMockDB) SaveObservation(project, agentID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Observations == nil {
		m.Observations = make(map[string][]db.Observation)
	}
	m.Observations[project] = append(m.Observations[project], db.Observation{
		AgentID: agentID,
		Content: content,
	})
	return nil
}

func (m *FaultToleranceMockDB) QueryHistory(project string, limit int) ([]db.Observation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obs, ok := m.Observations[project]
	if !ok {
		return []db.Observation{}, nil
	}
	if len(obs) > limit {
		return obs[len(obs)-limit:], nil
	}
	return obs, nil
}

func (m *FaultToleranceMockDB) SaveSpec(project, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Specs == nil {
		m.Specs = make(map[string]string)
	}
	m.Specs[project] = content
	return nil
}

func (m *FaultToleranceMockDB) GetSpec(project string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Specs == nil {
		return "", nil
	}
	return m.Specs[project], nil
}

func (m *FaultToleranceMockDB) SaveFeatures(project, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Features == nil {
		m.Features = make(map[string]string)
	}
	m.Features[project] = content

	// Also update FeatureList struct for orchestrator compatibility
	var fl db.FeatureList
	if err := json.Unmarshal([]byte(content), &fl); err == nil {
		m.FeatureList = fl
	}
	return nil
}

func (m *FaultToleranceMockDB) GetFeatures(project string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try map first
	if val, ok := m.Features[project]; ok {
		return val, nil
	}

	// Fallback to struct if populated (e.g. by direct assignment in test)
	if len(m.FeatureList.Features) > 0 {
		data, _ := json.Marshal(m.FeatureList)
		return string(data), nil
	}

	return "", nil
}

func (m *FaultToleranceMockDB) UpdateFeatureStatus(projectID, id string, status string, passes bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.FeatureList.Features {
		if m.FeatureList.Features[i].ID == id {
			m.FeatureList.Features[i].Status = status
			m.FeatureList.Features[i].Passes = passes
			return nil
		}
	}
	return fmt.Errorf("feature not found")
}

func (m *FaultToleranceMockDB) Close() error { return nil }
func (m *FaultToleranceMockDB) ReleaseAllLocks(projectID, agentID string) error { return nil }
func (m *FaultToleranceMockDB) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) { return true, nil }
func (m *FaultToleranceMockDB) ReleaseLock(projectID, path, agentID string) error { return nil }
func (m *FaultToleranceMockDB) GetActiveLocks(projectID string) ([]db.Lock, error) { return nil, nil }
func (m *FaultToleranceMockDB) Cleanup() error { return nil }
