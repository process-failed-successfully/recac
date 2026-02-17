package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBlockerKeys(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		ticket   map[string]interface{}
		expected []string
	}{
		{
			name:     "Nil fields",
			ticket:   map[string]interface{}{},
			expected: nil,
		},
		{
			name: "No issuelinks",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"summary": "Test Ticket",
				},
			},
			expected: nil,
		},
		{
			name: "With blocking link (not done)",
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
			name: "With blocking link (done)",
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
			name: "With non-blocking link",
			ticket: map[string]interface{}{
				"fields": map[string]interface{}{
					"issuelinks": []interface{}{
						map[string]interface{}{
							"type": map[string]interface{}{
								"inward": "relates to",
							},
							"inwardIssue": map[string]interface{}{
								"key": "REL-1",
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			name: "Multiple blockers mixed status",
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
										"name": "In Progress",
									},
								},
							},
						},
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
			expected: []string{"BLOCK-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.GetBlockerKeys(tt.ticket)
			assert.Equal(t, tt.expected, result)
		})
	}
}
