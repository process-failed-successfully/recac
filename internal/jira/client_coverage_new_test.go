package jira

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBlockerKeys(t *testing.T) {
	client := NewClient("http://jira.local", "user", "token")

	ticket := map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				// Blocker 1: In Progress (Blocked)
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
				// Blocker 2: Done (Not Blocked)
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
				// Other link type
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
	}

	blockers := client.GetBlockerKeys(ticket)
	assert.Len(t, blockers, 1)
	assert.Contains(t, blockers, "BLOCK-1")
	assert.NotContains(t, blockers, "BLOCK-2")
}
