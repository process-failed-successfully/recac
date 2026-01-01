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
