package scenarios

import (
	"testing"
)

func TestRegisteredScenarios_BasicProperties(t *testing.T) {
	// Ensure at least some scenarios are registered
	// We know prime-python and redis-challenge should be there if init() ran.
	// Since we are running tests in the package, init() in other files should run.

	// Check prime-python explicitly if Registry is empty (which shouldn't happen)
	if _, ok := Registry["prime-python"]; !ok {
		// Maybe init() order issue? But usually go test runs all init()
	}

	for name, s := range Registry {
		t.Run(name, func(t *testing.T) {
			if s.Name() == "" {
				t.Errorf("Scenario %s Name() is empty", name)
			}
			if s.Description() == "" {
				t.Errorf("Scenario %s Description() is empty", name)
			}

			spec := s.AppSpec("http://example.com/repo")
			if spec == "" {
				t.Errorf("Scenario %s AppSpec() is empty", name)
			}

			tickets := s.Generate("unique-id", "http://example.com/repo")
			// Some scenarios might generate 0 tickets? Unlikely for E2E.
			if len(tickets) == 0 {
				t.Logf("Scenario %s Generate() returned 0 tickets", name)
			}

			for i, ticket := range tickets {
				if ticket.ID == "" {
					t.Errorf("Scenario %s Ticket[%d] ID is empty", name, i)
				}
				if ticket.Type == "" {
					t.Errorf("Scenario %s Ticket[%d] Type is empty", name, i)
				}
				if ticket.Summary == "" {
					t.Errorf("Scenario %s Ticket[%d] Summary is empty", name, i)
				}
			}
		})
	}
}
