package scenarios

func init() {
	// Register a simple generic scenario to demonstrate the declarative config.
	// This scenario expects the agent to create a README.md file.
	Register(NewGenericScenario(GenericScenarioConfig{
		Name:        "simple-readme",
		Description: "Asks the agent to create a README.md file with specific content.",
		AppSpec:     "### ID:[README-1] Create README\n\nCreate a README.md file in the root of the repository. The README should contain the text 'Hello Recac E2E'.\n\nRepo: {{.RepoURL}}",
		Tickets: []TicketTemplate{
			{
				ID:      "README-1",
				Summary: "[{{.UniqueID}}] Create README",
				Desc:    "Create a file named README.md containing the text 'Hello Recac E2E'.\nRepo: {{.RepoURL}}",
				Type:    "Task",
			},
		},
		Validations: []ValidationStep{
			{
				Name: "Check README exists",
				Type: ValidateFileExists,
				Path: "README.md",
			},
			{
				Name:             "Check README content",
				Type:             ValidateFileContent,
				Path:             "README.md",
				ContentMustMatch: "Hello Recac E2E",
			},
		},
	}))
}
