package scenarios

import (
	"strings"
	"testing"
)

func TestRegistry_Scenarios(t *testing.T) {
	if len(Registry) == 0 {
		t.Fatal("Registry is empty")
	}

	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			// Test Name
			if scenario.Name() == "" {
				t.Error("Scenario Name() returned empty string")
			}
			if scenario.Name() != name {
				t.Errorf("Scenario Name() %s does not match registry key %s", scenario.Name(), name)
			}

			// Test Description
			if scenario.Description() == "" {
				t.Error("Scenario Description() returned empty string")
			}

			// Test AppSpec
			appSpec := scenario.AppSpec("http://example.com/repo")
			if appSpec == "" {
				t.Error("Scenario AppSpec() returned empty string")
			}
			if !strings.Contains(appSpec, "http://example.com/repo") {
				t.Error("Scenario AppSpec() does not contain repo URL")
			}

			// Test Generate
			specs := scenario.Generate("test-id", "http://example.com/repo")
			if len(specs) == 0 {
				t.Error("Scenario Generate() returned no tickets")
			}
			for _, spec := range specs {
				if spec.ID == "" {
					t.Error("Ticket Spec ID is empty")
				}
				if spec.Type == "" {
					t.Error("Ticket Spec Type is empty")
				}
				if spec.Summary == "" {
					t.Error("Ticket Spec Summary is empty")
				}
				if spec.Desc == "" {
					t.Error("Ticket Spec Desc is empty")
				}
			}
		})
	}
}
