package scenarios

import (
	"testing"
)

func TestRegistry_ContainsPrimePython(t *testing.T) {
	// init() functions should have run
	if _, ok := Registry["prime-python"]; !ok {
		t.Error("Registry does not contain 'prime-python' scenario")
	}
	if _, ok := Registry["simple-readme"]; !ok {
		t.Error("Registry does not contain 'simple-readme' scenario")
	}
}

func TestScenarios_Coverage(t *testing.T) {
	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			// Check Name
			if scenario.Name() == "" {
				t.Error("Scenario name is empty")
			}

			// Check Description
			if scenario.Description() == "" {
				t.Error("Scenario description is empty")
			}

			// Check Generate
			tickets := scenario.Generate("test-id", "http://repo.git")
			if len(tickets) == 0 {
				t.Logf("Scenario %s generated no tickets (this might be valid)", name)
			}

			// Check AppSpec
			spec := scenario.AppSpec("http://repo.git")
			if spec == "" {
				t.Error("Scenario AppSpec is empty")
			}

			// Check Verify
			// Create a temp dir to pass to Verify
			tmpDir := t.TempDir()

			// Most Verify implementations will fail because the directory is empty or doesn't match expectations.
			// But we just want to ensure it doesn't panic.
			// Some verify methods might panic if they don't check for errors, so we should recover.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Scenario %s Verify panicked: %v", name, r)
					}
				}()

				// We pass a dummy ticket map
				ticketKeys := map[string]string{}

				err := scenario.Verify(tmpDir, ticketKeys)
				// We expect error usually
				if err != nil {
					t.Logf("Scenario %s Verify failed as expected: %v", name, err)
				}
			}()
		})
	}
}
