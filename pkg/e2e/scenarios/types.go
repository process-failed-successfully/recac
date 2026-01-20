package scenarios

// TicketSpec defines a single Jira ticket to be created.
type TicketSpec struct {
	ID       string   // Internal ID for linking (e.g., "INIT", "CONFIG")
	Summary  string   // Summary of the ticket
	Desc     string   // Description of the ticket
	Type     string   // Issue Type (e.g., "Task", "Bug")
	Blockers []string // List of Internal IDs that block this ticket
	JiraKey  string   // Populated after creation (runtime)
}

// Scenario defines a set of tickets to generate for an E2E test.
type Scenario interface {
	// Name returns the unique name of the scenario.
	Name() string

	// Description returns a human-readable description.
	Description() string

	// Generate returns the list of tickets to create.
	// uniqueID is a timestamp or random string to make titles unique.
	// repoURL is the target repository URL for the agent.
	Generate(uniqueID string, repoURL string) []TicketSpec

	// AppSpec returns a textual application specification for the scenario.
	// This is used by the TPM agent to generate Jira tickets.
	AppSpec(repoURL string) string

	// Verify validates the results in the cloned repository.
	Verify(repoPath string, ticketKeys map[string]string) error
}

// Registry holds available scenarios.
var Registry = make(map[string]Scenario)

// Register adds a scenario to the registry.
func Register(s Scenario) {
	Registry[s.Name()] = s
}
