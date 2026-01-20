package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockJiraClient
type MockJiraClient struct {
	mock.Mock
}

func (m *MockJiraClient) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	args := m.Called(ctx, projectKey, summary, description, issueType, labels)
	return args.String(0), args.Error(1)
}

func (m *MockJiraClient) CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
	args := m.Called(ctx, projectKey, summary, description, issueType, parentKey, labels)
	return args.String(0), args.Error(1)
}

func (m *MockJiraClient) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	args := m.Called(ctx, inwardKey, outwardKey, linkType)
	return args.Error(0)
}

// MockAgent
type MockAgent struct {
	mock.Mock
}

func (m *MockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestGenerateTickets(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	specContent := "App Spec"
	projectKey := "PROJ"
	labels := []string{"label1"}

	// Mock Agent Response
	tickets := []ticketNode{
		{
			Title:              "Epic 1",
			Description:        "Repo: https://github.com/example/repo\nDescription of Epic 1",
			Type:               "Epic",
			AcceptanceCriteria: []string{"AC1"},
			Children: []ticketNode{
				{
					Title:              "Story 1",
					Description:        "Repo: https://github.com/example/repo\nDescription of Story 1",
					Type:               "Story",
					AcceptanceCriteria: []string{"AC2"},
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	// Mock Jira Interactions
	mockJira.On("CreateTicket", mock.Anything, projectKey, "Epic 1", mock.Anything, "Epic", labels).Return("PROJ-1", nil)
	mockJira.On("CreateChildTicket", mock.Anything, projectKey, "Story 1", mock.Anything, "Story", "PROJ-1", labels).Return("PROJ-2", nil)

	_, err := generateTickets(context.Background(), specContent, projectKey, labels, mockJira, mockAgent)
	assert.NoError(t, err)

	mockJira.AssertExpectations(t)
	mockAgent.AssertExpectations(t)
}

func TestGenerateTickets_AgentFailure(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	mockAgent.On("Send", mock.Anything, mock.Anything).Return("", assert.AnError)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.Error(t, err)
}

func TestGenerateTickets_InvalidRepo(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Epic 1",
			Description: "Missing Repo URL", // Invalid
			Type:        "Epic",
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing repository URL")
}

func TestGenerateTickets_InvalidJSON(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	mockAgent.On("Send", mock.Anything, mock.Anything).Return("not json", nil)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse agent response")
}

func TestGenerateTickets_JiraCreateError(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Epic 1",
			Description: "Repo: https://example.com",
			Type:        "Epic",
			Children: []ticketNode{
				{
					Title:       "Story 1",
					Description: "Repo: https://example.com",
					Type:        "Story",
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	// Mock Jira Failure for Epic
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Epic 1", mock.Anything, "Epic", mock.Anything).Return("", assert.AnError)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.NoError(t, err) // It logs error but returns nil

	mockJira.AssertExpectations(t)
}

func TestGenerateTickets_ChildAndLinkLogic(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Epic 1",
			Description: "Repo: https://example.com",
			Type:        "Epic",
			BlockedBy:   []string{"Blocker Ticket"}, // External blocker, won't be in map, so skipped?
			// Need internal dependency to test linking logic
			Children: []ticketNode{
				{
					Title:       "Story 1", // Will fail
					Description: "Repo: https://example.com",
					Type:        "Story",
				},
				{
					Title:       "Story 2", // Will succeed
					Description: "Repo: https://example.com",
					Type:        "Story",
					BlockedBy:   []string{"Story 1"}, // Internal blocker
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	// Epic Success
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Epic 1", mock.Anything, "Epic", mock.Anything).Return("PROJ-1", nil)

	// Story 1 Fail
	mockJira.On("CreateChildTicket", mock.Anything, "PROJ", "Story 1", mock.Anything, "Story", "PROJ-1", mock.Anything).Return("", assert.AnError)

	// Story 2 Success
	mockJira.On("CreateChildTicket", mock.Anything, "PROJ", "Story 2", mock.Anything, "Story", "PROJ-1", mock.Anything).Return("PROJ-2", nil)

	// Let's add another independent epic to test linking
	ticketsWithBlocker := []ticketNode{
		{
			Title:       "Blocker Epic",
			Description: "Repo: https://example.com",
			Type:        "Epic",
		},
		{
			Title:       "Blocked Epic",
			Description: "Repo: https://example.com",
			Type:        "Epic",
			BlockedBy:   []string{"Blocker Epic"},
		},
	}
	jsonBytes2, _ := json.Marshal(ticketsWithBlocker)
	mockAgent.ExpectedCalls = nil // Clear previous expectation
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes2), nil)

	// Re-mock Jira for second run
	mockJira.ExpectedCalls = nil
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Blocker Epic", mock.Anything, "Epic", mock.Anything).Return("PROJ-10", nil)
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Blocked Epic", mock.Anything, "Epic", mock.Anything).Return("PROJ-11", nil)

	// Expect Link
	mockJira.On("AddIssueLink", mock.Anything, "PROJ-10", "PROJ-11", "Blocks").Return(nil)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.NoError(t, err)

	mockJira.AssertExpectations(t)
}

func TestGenerateTickets_LinkError(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Blocker",
			Description: "Repo: https://example.com",
			Type:        "Epic",
		},
		{
			Title:       "Blocked",
			Description: "Repo: https://example.com",
			Type:        "Epic",
			BlockedBy:   []string{"Blocker"},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Blocker", mock.Anything, "Epic", mock.Anything).Return("PROJ-1", nil)
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Blocked", mock.Anything, "Epic", mock.Anything).Return("PROJ-2", nil)

	// Mock Link Failure
	mockJira.On("AddIssueLink", mock.Anything, "PROJ-1", "PROJ-2", "Blocks").Return(assert.AnError)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.NoError(t, err) // Should continue despite link error

	mockJira.AssertExpectations(t)
}

func TestGenerateTickets_Defaults(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Epic 1",
			Description: "Repo: https://example.com",
			Type:        "", // Empty type, should default to Epic
			Children: []ticketNode{
				{
					Title:       "Story 1",
					Description: "Repo: https://example.com",
					Type:        "", // Empty type, should default to Story
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	// Verify "Epic" string is passed
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Epic 1", mock.Anything, "Epic", mock.Anything).Return("PROJ-1", nil)
	// Verify "Story" string is passed
	mockJira.On("CreateChildTicket", mock.Anything, "PROJ", "Story 1", mock.Anything, "Story", "PROJ-1", mock.Anything).Return("PROJ-2", nil)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.NoError(t, err)

	mockJira.AssertExpectations(t)
}

func TestGenerateTickets_MarkdownStripping(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Epic",
			Description: "Repo: https://example.com",
			Type:        "Epic",
		},
	}
	jsonBytes, _ := json.Marshal(tickets)

	// Test Case 1: JSON block with "json" language identifier
	jsonStr1 := "Here is the plan:\n```json\n" + string(jsonBytes) + "\n```"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonStr1, nil).Once()
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Epic", mock.Anything, "Epic", mock.Anything).Return("PROJ-1", nil).Once()

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.NoError(t, err)

	// Test Case 2: Generic code block
	jsonStr2 := "Here is the plan:\n```\n" + string(jsonBytes) + "\n```"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonStr2, nil).Once()
	mockJira.On("CreateTicket", mock.Anything, "PROJ", "Epic", mock.Anything, "Epic", mock.Anything).Return("PROJ-2", nil).Once()

	_, err = generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.NoError(t, err)

	mockJira.AssertExpectations(t)
}

func TestGenerateTickets_StoryInvalidRepo(t *testing.T) {
	mockJira := new(MockJiraClient)
	mockAgent := new(MockAgent)

	tickets := []ticketNode{
		{
			Title:       "Epic 1",
			Description: "Repo: https://example.com",
			Type:        "Epic",
			Children: []ticketNode{
				{
					Title:       "Story 1",
					Description: "Missing Repo", // Invalid
					Type:        "Story",
				},
			},
		},
	}
	jsonBytes, _ := json.Marshal(tickets)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	_, err := generateTickets(context.Background(), "spec", "PROJ", []string{}, mockJira, mockAgent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing repository URL")
}
