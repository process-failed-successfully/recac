package scenarios

import (
	"testing"
)

func TestAllScenarios(t *testing.T) {
	if len(Registry) == 0 {
		t.Fatal("Registry is empty. init() functions might not have run or no scenarios registered.")
	}

	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			// Test Name
			if n := scenario.Name(); n == "" {
				t.Error("Name() should not be empty")
			}

			// Test Description
			if d := scenario.Description(); d == "" {
				t.Error("Description() should not be empty")
			}

			// Test AppSpec
			spec := scenario.AppSpec("https://github.com/example/repo")
			if spec == "" {
				t.Error("AppSpec() should not be empty")
			}

			// Test Generate
			tickets := scenario.Generate("TEST-ID", "https://github.com/example/repo")
			if len(tickets) == 0 {
				t.Error("Generate() should return at least one ticket")
			}

			for _, ticket := range tickets {
				if ticket.ID == "" {
					t.Error("Ticket ID should not be empty")
				}
				if ticket.Summary == "" {
					t.Error("Ticket Summary should not be empty")
				}
				if ticket.Desc == "" {
					t.Error("Ticket Desc should not be empty")
				}
				if ticket.Type == "" {
					t.Error("Ticket Type should not be empty")
				}
			}
		})
	}
}
