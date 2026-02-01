package scenarios

import (
	"testing"
)

func TestAllScenarios_Basics(t *testing.T) {
	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			if scenario.Name() == "" {
				t.Error("Name() should not be empty")
			}
			if scenario.Description() == "" {
				t.Error("Description() should not be empty")
			}
			if scenario.AppSpec("repo") == "" {
				t.Error("AppSpec() should not be empty")
			}
			if len(scenario.Generate("id", "repo")) == 0 {
				t.Error("Generate() should return tickets")
			}
		})
	}
}
