package state

import (
	"encoding/json"
	"os"
)

// E2EContext holds the state of an active E2E test session.
// It allows different steps (setup, deploy, verify) to share data.
type E2EContext struct {
	// ID is a unique identifier for this run (often used for temp dirs)
	ID string `json:"id"`

	// ScenarioName is the name of the scenario being run
	ScenarioName string `json:"scenario_name"`

	// Jira State
	JiraProjectKey string            `json:"jira_project_key"`
	JiraLabel      string            `json:"jira_label"`
	TicketMap      map[string]string `json:"ticket_map"` // Key (e.g., "PRIMES") -> IssueID (e.g., "PROJ-123")

	// Repo State
	RepoURL   string `json:"repo_url"`
	Branch    string `json:"branch"` // Logic might differ, but usually we care about the remote branch
	AuthToken string `json:"-"`      // Don't persist sensitive tokens to disk if avoidable, or encrypt

	// Deployment State
	Namespace   string `json:"namespace"`
	ReleaseName string `json:"release_name"`
	Provider    string `json:"provider"`
	Model       string `json:"model"`
}

// Load reads the context from a file
func Load(path string) (*E2EContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ctx E2EContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, err
	}
	return &ctx, nil
}

// Save writes the context to a file
func (c *E2EContext) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
