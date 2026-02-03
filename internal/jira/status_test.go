package jira

import (
	"testing"
)

func TestClient_GetBlockerKeys_StatusHandling(t *testing.T) {
	client := &Client{} // We don't need real HTTP client for this logic test

	tests := []struct {
		name          string
		ticket        map[string]interface{}
		expectBlocked bool
	}{
		{
			name: "Blocked by To Do",
			ticket: createTicketWithBlocker("PROJ-2", "To Do"),
			expectBlocked: true,
		},
		{
			name: "Blocked by In Progress",
			ticket: createTicketWithBlocker("PROJ-3", "In Progress"),
			expectBlocked: true,
		},
		{
			name: "Not Blocked by Done",
			ticket: createTicketWithBlocker("PROJ-4", "Done"),
			expectBlocked: false,
		},
		{
			name: "Not Blocked by Cancelled",
			ticket: createTicketWithBlocker("PROJ-5", "Cancelled"),
			expectBlocked: false,
		},
		{
			name: "Not Blocked by Released",
			ticket: createTicketWithBlocker("PROJ-6", "Released"),
			expectBlocked: false,
		},
		{
			name: "Not Blocked by Deployed",
			ticket: createTicketWithBlocker("PROJ-7", "Deployed"),
			expectBlocked: false,
		},
		{
			name: "Not Blocked by Passed",
			ticket: createTicketWithBlocker("PROJ-8", "Passed"),
			expectBlocked: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockers := client.GetBlockerKeys(tt.ticket)
			if tt.expectBlocked {
				if len(blockers) == 0 {
					t.Errorf("Expected blockers, got none")
				}
			} else {
				if len(blockers) > 0 {
					t.Errorf("Expected no blockers, got %v", blockers)
				}
			}
		})
	}
}

func createTicketWithBlocker(blockerKey, status string) map[string]interface{} {
	return map[string]interface{}{
		"fields": map[string]interface{}{
			"issuelinks": []interface{}{
				map[string]interface{}{
					"type": map[string]interface{}{
						"inward": "is blocked by",
					},
					"inwardIssue": map[string]interface{}{
						"key": blockerKey,
						"fields": map[string]interface{}{
							"status": map[string]interface{}{
								"name": status,
							},
						},
					},
				},
			},
		},
	}
}
