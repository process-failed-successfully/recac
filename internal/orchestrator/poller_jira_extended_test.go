package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractRequiredFeatures_Direct(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string // Just checking descriptions for simplicity
	}{
		{
			name: "Basic Features",
			input: `
REQUIRED FEATURES:
- Feature A
- Feature B
`,
			expected: []string{"Feature A", "Feature B"},
		},
		{
			name: "Acceptance Criteria",
			input: `
ACCEPTANCE CRITERIA:
* Criteria 1
* Criteria 2
`,
			expected: []string{"Criteria 1", "Criteria 2"},
		},
		{
			name: "Mixed Bullets",
			input: `
REQUIRED FEATURES:
- Feature A
* Feature B
`,
			expected: []string{"Feature A", "Feature B"},
		},
		{
			name: "Stop at Next Section",
			input: `
REQUIRED FEATURES:
- Feature A

OTHER SECTION:
- Feature B
`,
			expected: []string{"Feature A"},
		},
		{
			name: "Case Insensitive Header",
			input: `
required features:
- Feature A
`,
			expected: []string{"Feature A"},
		},
		{
			name: "Optional Colon",
			input: `
REQUIRED FEATURES
- Feature A
`,
			expected: []string{"Feature A"},
		},
		{
			name: "Whitespace in Header",
			input: `
   REQUIRED FEATURES:
- Feature A
`,
			expected: []string{"Feature A"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			features := extractRequiredFeatures(tc.input)
			var descriptions []string
			for _, f := range features {
				descriptions = append(descriptions, f.Description)
				// Also check slug format if needed
				if f.Description == "My Feature!" {
					assert.Equal(t, "req-my-feature", f.ID)
				}
			}
			assert.Equal(t, tc.expected, descriptions)
		})
	}

	t.Run("Slug Generation", func(t *testing.T) {
		input := "REQUIRED FEATURES:\n- My Feature!"
		features := extractRequiredFeatures(input)
		assert.Len(t, features, 1)
		assert.Equal(t, "My Feature!", features[0].Description)
		assert.Equal(t, "req-my-feature", features[0].ID)
	})
}

func TestExtractRequiredFeatures_Concurrency(t *testing.T) {
	input := `
REQUIRED FEATURES:
- Feature A
- Feature B
`
	// Run parallel goroutines to trigger race conditions if any
	const numGoroutines = 100
	done := make(chan bool)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			features := extractRequiredFeatures(input)
			if len(features) != 2 {
				t.Errorf("expected 2 features, got %d", len(features))
			}
			done <- true
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}
