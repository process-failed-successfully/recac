package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTruncateString(t *testing.T) {
	assert.Equal(t, "abc", truncateString("abc", 5))
	assert.Equal(t, "abcde", truncateString("abcde", 5))
	assert.Equal(t, "ab...", truncateString("abcdef", 5))
}

func TestCreateTicketsFromNodes(t *testing.T) {
	ctx := context.Background()
	// MockJiraClient is defined in tickets_test.go
	mockJira := new(MockJiraClient)

	// Define nodes structure matching ticketNode
	nodes := []ticketNode{
		{
			Title:       "ID:[EPIC-1] Epic Title",
			Description: "Epic Description\nRepo: http://example.com", // Add Repo
			Type:        "Epic",
			Children: []ticketNode{
				{
					Title:       "Story Title",
					Description: "Story Description\nRepo: http://example.com", // Add Repo
					Type:        "Story",
					BlockedBy:   []string{"ID:[EPIC-1] Epic Title"},
				},
			},
		},
	}

	// 1. Create Epic
	mockJira.On("CreateTicket", ctx, "PROJ", "ID:[EPIC-1] Epic Title", mock.Anything, "Epic", mock.Anything).Return("PROJ-1", nil)

	// 2. Create Story (child of PROJ-1)
	mockJira.On("CreateChildTicket", ctx, "PROJ", "Story Title", mock.Anything, "Story", "PROJ-1", mock.Anything).Return("PROJ-2", nil)

	// 3. Link Blockers
	mockJira.On("AddIssueLink", ctx, "PROJ-1", "PROJ-2", "Blocks").Return(nil)

	mapping, err := createTicketsFromNodes(ctx, nodes, "PROJ", "", []string{}, mockJira)
	assert.NoError(t, err)

	assert.Equal(t, "PROJ-1", mapping["EPIC-1"])
	mockJira.AssertExpectations(t)
}
