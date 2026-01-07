package db

import "time"

type FeatureDependencies struct {
	DependsOnIDs        []string `json:"depends_on_ids"`
	ExclusiveWritePaths []string `json:"exclusive_write_paths"`
	ReadOnlyPaths       []string `json:"read_only_paths"`
}

type Feature struct {
	ID           string              `json:"id"`
	Category     string              `json:"category"`
	Priority     string              `json:"priority"` // "POC", "MVP", "Production"
	Description  string              `json:"description"`
	Status       string              `json:"status"`
	Passes       bool                `json:"passes"`
	Steps        []string            `json:"steps"`
	Dependencies FeatureDependencies `json:"dependencies"`
}

type Lock struct {
	Path      string    `json:"path"`
	AgentID   string    `json:"agent_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type FeatureList struct {
	ProjectName string    `json:"project_name"`
	Features    []Feature `json:"features"`
}

// Observation represents a recorded event or fact
type Observation struct {
	ID        int64     `json:"id"`
	AgentID   string    `json:"agent_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Store interface defines the methods for persistent storage
type Store interface {
	Close() error
	SaveObservation(projectID, agentID, content string) error
	QueryHistory(projectID string, limit int) ([]Observation, error)
	SetSignal(key, value string) error
	GetSignal(key string) (string, error)
	DeleteSignal(key string) error
	SaveFeatures(features string) error // JSON blob for flexibility
	GetFeatures() (string, error)
	UpdateFeatureStatus(id string, status string, passes bool) error

	// Locking methods
	AcquireLock(path, agentID string, timeout time.Duration) (bool, error)
	ReleaseLock(path, agentID string) error
	ReleaseAllLocks(agentID string) error
	GetActiveLocks() ([]Lock, error)

	// Maintenance
	Cleanup() error
}
