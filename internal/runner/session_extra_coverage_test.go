package runner

import (
	"os"
	"path/filepath"
	"recac/internal/db"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type MinimalMockDB struct {
	Spec string
}

func (m *MinimalMockDB) Close() error { return nil }
func (m *MinimalMockDB) SaveObservation(projectID, agentID, content string) error { return nil }
func (m *MinimalMockDB) QueryHistory(projectID string, limit int) ([]db.Observation, error) { return nil, nil }
func (m *MinimalMockDB) SetSignal(projectID, key, value string) error { return nil }
func (m *MinimalMockDB) GetSignal(projectID, key string) (string, error) { return "", nil }
func (m *MinimalMockDB) DeleteSignal(projectID, key string) error { return nil }
func (m *MinimalMockDB) SaveFeatures(projectID string, features string) error { return nil }
func (m *MinimalMockDB) GetFeatures(projectID string) (string, error) { return "", nil }
func (m *MinimalMockDB) SaveSpec(projectID string, spec string) error { return nil }
func (m *MinimalMockDB) GetSpec(projectID string) (string, error) { return m.Spec, nil }
func (m *MinimalMockDB) UpdateFeatureStatus(projectID, id string, status string, passes bool) error { return nil }
func (m *MinimalMockDB) AcquireLock(projectID, path, agentID string, timeout time.Duration) (bool, error) { return true, nil }
func (m *MinimalMockDB) ReleaseLock(projectID, path, agentID string) error { return nil }
func (m *MinimalMockDB) ReleaseAllLocks(projectID, agentID string) error { return nil }
func (m *MinimalMockDB) GetActiveLocks(projectID string) ([]db.Lock, error) { return nil, nil }
func (m *MinimalMockDB) Cleanup() error { return nil }

// TestSession_ReadSpec_Fallbacks covers scenarios where file is missing but content exists in memory or DB.
func TestSession_ReadSpec_Fallbacks(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Fallback: SpecContent (memory)
	t.Run("SpecContent fallback", func(t *testing.T) {
		specContent := "Spec from Memory"
		// Create session with SpecContent but NO file
		session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
		session.SpecContent = specContent
		session.Project = "test-project"

		content, err := session.ReadSpec()
		assert.NoError(t, err)
		assert.Equal(t, specContent, content)

		// Verify file was created
		fileContent, err := os.ReadFile(filepath.Join(tmpDir, "app_spec.txt"))
		assert.NoError(t, err)
		assert.Equal(t, specContent, string(fileContent))
	})

	// 2. Fallback: DB
	t.Run("DB fallback", func(t *testing.T) {
		specContent := "Spec from DB"
		tmpDirDB := t.TempDir()

		// Setup MockDB
		mockDB := &MinimalMockDB{
			Spec: specContent,
		}

		session := NewSession(nil, &MockAgent{}, tmpDirDB, "alpine", "test-project-db", "gemini", "gemini-pro", 1)
		session.DBStore = mockDB
		session.Project = "test-project-db"

		content, err := session.ReadSpec()
		assert.NoError(t, err)
		assert.Equal(t, specContent, content)

		// Verify file was created
		fileContent, err := os.ReadFile(filepath.Join(tmpDirDB, "app_spec.txt"))
		assert.NoError(t, err)
		assert.Equal(t, specContent, string(fileContent))
	})
}

func TestSession_InitializeAgentState_Nil(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)

	// Force StateManager to nil
	session.StateManager = nil

	err := session.InitializeAgentState(100)
	assert.NoError(t, err)
}

func TestSession_LoadAgentState_Nil(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.StateManager = nil
	err := session.LoadAgentState()
	assert.NoError(t, err)
}

func TestSession_SaveAgentState_Nil(t *testing.T) {
	tmpDir := t.TempDir()
	session := NewSession(nil, &MockAgent{}, tmpDir, "alpine", "test-project", "gemini", "gemini-pro", 1)
	session.StateManager = nil
	err := session.SaveAgentState()
	assert.NoError(t, err)
}
