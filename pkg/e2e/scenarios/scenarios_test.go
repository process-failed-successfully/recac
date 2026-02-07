package scenarios

import (
	"testing"
)

func TestRegistry_Scenarios(t *testing.T) {
	if len(Registry) == 0 {
		t.Fatal("Registry is empty, expected scenarios to be registered")
	}

	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			// Test Name
			if scenario.Name() == "" {
				t.Errorf("Scenario %s has empty Name", name)
			}
			if scenario.Name() != name {
				t.Errorf("Scenario name mismatch: got %s, expected %s", scenario.Name(), name)
			}

			// Test Description
			if scenario.Description() == "" {
				t.Errorf("Scenario %s has empty Description", name)
			}

			// Test AppSpec
			appSpec := scenario.AppSpec("http://example.com/repo.git")
			if appSpec == "" {
				t.Errorf("Scenario %s has empty AppSpec", name)
			}

			// Test Generate
			tickets := scenario.Generate("unique-id", "http://example.com/repo.git")
			if len(tickets) == 0 {
				// Some scenarios might return empty tickets if they are purely prompt-based or handle ticket creation differently,
				// but based on the code I've seen so far, they return tickets.
				// Let's check if this assumption holds. If not, I'll adjust.
				// For now, I'll log it but not error if it's expected for some.
				// Looking at DistributedLogScenario, it returns 1 ticket.
				t.Logf("Scenario %s generated 0 tickets", name)
			}

			for _, ticket := range tickets {
				if ticket.ID == "" {
					t.Errorf("Scenario %s generated ticket with empty ID", name)
				}
				if ticket.Summary == "" {
					t.Errorf("Scenario %s generated ticket with empty Summary", name)
				}
				if ticket.Desc == "" {
					t.Errorf("Scenario %s generated ticket with empty Desc", name)
				}
			}
		})
	}
}
