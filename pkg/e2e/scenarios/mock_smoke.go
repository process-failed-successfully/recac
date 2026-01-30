package scenarios

func init() {
	// Register a simple smoke test scenario.
	// It runs a basic echo command to verify the agent can execute commands.
	Register(NewGenericScenario(GenericScenarioConfig{
		Name:        "mock-smoke",
		Description: "Smoke test to verify agent connectivity and basic command execution.",
		AppSpec:     "### ID:[SMOKE-1] Smoke Test\n\nRun a simple echo command to verify connectivity.\n\nRepo: {{.RepoURL}}",
		Tickets: []TicketTemplate{
			{
				ID:      "SMOKE-1",
				Summary: "[{{.UniqueID}}] Smoke Test",
				Desc:    "Run 'echo hello world' in the repository root.\nRepo: {{.RepoURL}}",
				Type:    "Task",
			},
		},
		Validations: []ValidationStep{
			{
				Name: "Verify Command Execution",
				Type: ValidateRunCommand,
				Path: "echo",
				Args: []string{"hello", "world"},
				ContentMustMatch: "hello world",
			},
		},
	}))
}
