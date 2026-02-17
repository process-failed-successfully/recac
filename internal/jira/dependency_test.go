package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveDependencies(t *testing.T) {
	tests := []struct {
		name         string
		issues       []map[string]interface{}
		dependencies map[string][]string // Key -> []BlockerKeys
		expected     []string            // Expected order
		wantErr      bool
	}{
		{
			name: "No Dependencies",
			issues: []map[string]interface{}{
				{"key": "A"},
				{"key": "B"},
				{"key": "C"},
			},
			dependencies: map[string][]string{},
			expected:     []string{"A", "B", "C"}, // Sorted alphabetically due to Kahn's algo queue sort
		},
		{
			name: "Simple Chain A->B->C (A blocks B, B blocks C)",
			issues: []map[string]interface{}{
				{"key": "C"},
				{"key": "A"},
				{"key": "B"},
			},
			dependencies: map[string][]string{
				"B": {"A"}, // B is blocked by A
				"C": {"B"}, // C is blocked by B
			},
			expected: []string{"A", "B", "C"},
		},
		{
			name: "Diamond Dependency A->(B,C)->D",
			issues: []map[string]interface{}{
				{"key": "D"},
				{"key": "B"},
				{"key": "C"},
				{"key": "A"},
			},
			dependencies: map[string][]string{
				"B": {"A"},
				"C": {"A"},
				"D": {"B", "C"},
			},
			expected: []string{"A", "B", "C", "D"}, // B and C order might vary but usually alphabetical with my sort
		},
		{
			name: "Circular Dependency A->B->A",
			issues: []map[string]interface{}{
				{"key": "A"},
				{"key": "B"},
			},
			dependencies: map[string][]string{
				"B": {"A"},
				"A": {"B"},
			},
			wantErr: true,
		},
		{
			name: "Missing Dependency (Ignored)",
			issues: []map[string]interface{}{
				{"key": "B"},
			},
			dependencies: map[string][]string{
				"B": {"A"}, // A is not in the list
			},
			expected: []string{"B"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetcher := func(issue map[string]interface{}) ([]string, error) {
				key := issue["key"].(string)
				return tt.dependencies[key], nil
			}

			got, err := ResolveDependencies(tt.issues, fetcher)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveDependencies() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				var gotKeys []string
				for _, i := range got {
					gotKeys = append(gotKeys, i["key"].(string))
				}
				assert.Equal(t, tt.expected, gotKeys)
			}
		})
	}
}
