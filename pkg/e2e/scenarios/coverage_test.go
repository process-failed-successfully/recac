package scenarios

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScenariosCoverage(t *testing.T) {
	// Ensure we have scenarios in registry
	// Depending on initialization order, we might need to rely on them being registered.
	// In Go, init() in same package are run.

	for name, s := range Registry {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, s.Name())
			assert.NotPanics(t, func() { s.Description() })
			assert.NotPanics(t, func() { s.AppSpec("http://repo") })

			// Generate usually just returns a struct, safe to call
			assert.NotPanics(t, func() {
				s.Generate("test-id", "http://repo")
			})

			// Verify usually checks files, so it will likely fail, but shouldn't panic
			// We pass a temp dir
			tmpDir := t.TempDir()
			err := s.Verify(tmpDir, nil)
			// We don't care if it returns error, just that it doesn't panic
			_ = err
		})
	}
}
