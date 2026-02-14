package main

import (
	"context"
	"encoding/json"
	"recac/internal/jira"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerateTickets_DryRun(t *testing.T) {
	// Setup DryRunClient
	dryRunClient := jira.NewDryRunClient()
	mockAgent := new(MockAgent)

	specContent := "App Spec"
	projectKey := "PROJ"
	labels := []string{"label1"}
	repoURL := "https://github.com/example/repo"

	// Mock Agent Response
	tickets := []ticketNode{
		{
			Title:              "Epic 1",
			Description:        "Description of Epic 1", // Repo URL injected by logic
			Type:               "Epic",
			Children: []ticketNode{
				{
					Title:              "Story 1",
					Description:        "Description of Story 1",
					Type:               "Story",
					Children: []ticketNode{
						{
							Title: "Subtask 1",
							Description: "Do work",
							Type: "Subtask",
						},
					},
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	// Execute with DryRunClient
	mapping, err := generateTickets(context.Background(), specContent, projectKey, repoURL, labels, dryRunClient, mockAgent)

	assert.NoError(t, err)
	// Mapping might be empty if no ID:[...] tags are present, which is expected here.
	// assert.NotEmpty(t, mapping)

	// Verify Fake IDs are in mapping
	// Since mapping is based on ID:[XXX] in title, and we didn't put any ID tags in titles, mapping might be empty.
	// Let's add ID tags to titles to verify mapping.

	// Reset and try with IDs
	mockAgent = new(MockAgent)
	ticketsWithIDs := []ticketNode{
		{
			Title: "ID:[EPIC-1] Epic Title",
			Description: "Desc",
			Type: "Epic",
		},
	}
	jsonBytes2, _ := json.Marshal(ticketsWithIDs)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes2), nil)

	// We need a new dry run client to reset counter if we care about specific IDs, but we don't.
	dryRunClient = jira.NewDryRunClient()

	mapping, err = generateTickets(context.Background(), specContent, projectKey, repoURL, labels, dryRunClient, mockAgent)
	assert.NoError(t, err)

	// Check if ID was mapped to a fake key
	assert.Contains(t, mapping, "EPIC-1")
	assert.True(t, strings.HasPrefix(mapping["EPIC-1"], "DRY-"), "Mapped key should start with DRY-")

	mockAgent.AssertExpectations(t)
}
