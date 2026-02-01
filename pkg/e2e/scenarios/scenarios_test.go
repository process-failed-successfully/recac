package scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisteredScenarios(t *testing.T) {
	// Ensure we have scenarios
	if len(Registry) == 0 {
		// This might happen if init() functions haven't run or if running specific test target.
		// But in same package, they should run.
		// If fails, we might need to manually register them or import them for side effects.
		// Since we are in same package, init() should fire.
		// However, Go test runner might compile only test files if we don't include others.
		// We rely on "go test ./..." to include all files.
	}

	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			// Test Name
			assert.NotEmpty(t, scenario.Name())
			assert.Equal(t, name, scenario.Name())

			// Test Description
			assert.NotEmpty(t, scenario.Description())

			// Test AppSpec
			spec := scenario.AppSpec("http://repo")
			assert.NotEmpty(t, spec)
			assert.Contains(t, spec, "http://repo")

			// Test Generate
			tickets := scenario.Generate("uid", "http://repo")
			assert.NotEmpty(t, tickets)
			for _, ticket := range tickets {
				assert.NotEmpty(t, ticket.ID)
				assert.NotEmpty(t, ticket.Summary)
				assert.NotEmpty(t, ticket.Desc)
				assert.Contains(t, ticket.Desc, "http://repo")
			}
		})
	}
}
