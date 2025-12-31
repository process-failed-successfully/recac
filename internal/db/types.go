package db

type FeatureDependencies struct {
	DependsOnIDs        []string `json:"depends_on_ids"`
	ExclusiveWritePaths []string `json:"exclusive_write_paths"`
	ReadOnlyPaths       []string `json:"read_only_paths"`
}

type Feature struct {
	ID           string              `json:"id"`
	Category     string              `json:"category"`
	Description  string              `json:"description"`
	Status       string              `json:"status"`
	Passes       bool                `json:"passes"`
	Steps        []string            `json:"steps"`
	Dependencies FeatureDependencies `json:"dependencies"`
}

type FeatureList struct {
	ProjectName string    `json:"project_name"`
	Features    []Feature `json:"features"`
}
