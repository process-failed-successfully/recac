package jira

import (
	"reflect"
	"testing"
)

func TestGetBlockerKeys(t *testing.T) {
	client := &Client{} // Client methods used here don't use the struct fields, so empty client is fine

	tests := []struct {
		name     string
		ticket   map[string]interface{}
		expected []string
	}{
		{
			name: "No Links",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{},
				},
			},
			expected: nil,
		},
		{
			name: "Irrelevant Link Type",
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
			name: "Blocked By - Done Issue",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "PROJ-1",
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
			expected: nil, // Should be empty because blocker is Done
		},
		{
			name: "Blocked By - Active Issue",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "is blocked by",
							},
							"inwardIssue": map[string]interface{}{
								"key": "PROJ-2",
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
			expected: []string{"PROJ-2"},
		},
		{
			name: "Mixed Links",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{ // Valid Blocker
							"type": map[string]interface{}{"inward": "is blocked by"},
							"inwardIssue": map[string]interface{}{
								"key": "PROJ-3",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{"name": "To Do"},
								},
							},
						},
						map[string]interface{}{ // Done Blocker (ignored)
							"type": map[string]interface{}{"inward": "is blocked by"},
							"inwardIssue": map[string]interface{}{
								"key": "PROJ-4",
								"fields": map[string]interface{}{
									"status": map[string]interface{}{"name": "Done"},
								},
							},
						},
						map[string]interface{}{ // Other Link Type (ignored)
							"type": map[string]interface{}{"inward": "relates to"},
							"inwardIssue": map[string]interface{}{
								"key": "PROJ-5",
							},
						},
					},
				},
			},
			expected: []string{"PROJ-3"},
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
