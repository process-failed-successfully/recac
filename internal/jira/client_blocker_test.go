package jira

import (
	"reflect"
	"testing"
)

func TestGetBlockerKeys(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	tests := []struct {
		name     string
		ticket   map[string]interface{}
		expected []string
	}{
		{
			name: "No Blockers",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{},
				},
			},
			expected: nil,
		},
		{
			name: "With Blockers - Not Done",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "BLOCK-1",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "To Do",
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"BLOCK-1"},
		},
		{
			name: "With Blockers - Done",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "BLOCK-2",
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
		{
			name: "With Non-Blocker Links",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "relates to",
							},
							"inwardIssue": map[string]interface{}{
								"key": "REL-1",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{
										"name": "To Do",
									},
								},
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Malformed Data",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": "invalid",
				},
			},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.GetBlockerKeys(tt.ticket)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("GetBlockerKeys() = %v, want %v", got, tt.expected)
			}
		})
	}
}
