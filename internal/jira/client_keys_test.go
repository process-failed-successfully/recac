package jira

import (
	"testing"
)

func TestGetBlockerKeys(t *testing.T) {
	client := NewClient("", "", "")

	tests := []struct {
		name     string
		ticket   map[string]interface{}
		expected []string
	}{
		{
			name: "No links",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{},
			},
			expected: nil,
		},
		{
			name: "No blockers",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "relates to",
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Unresolved blocker",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "RD-158",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "In Progress",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"RD-158"},
		},
		{
			name: "Resolved blocker",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "RD-159",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "Done",
									},
								},
							},
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockerKeys := client.GetBlockerKeys(tt.ticket)
			if len(blockerKeys) != len(tt.expected) {
				t.Errorf("expected %d blocker keys, got %d", len(tt.expected), len(blockerKeys))
			}
			for i, k := range blockerKeys {
				if k != tt.expected[i] {
					t.Errorf("expected blocker key %q, got %q", tt.expected[i], k)
				}
			}
		})
	}
}
