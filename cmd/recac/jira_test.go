package main

import (
	"context"
	"strings"
	"testing"
)

// Mock Jira Client for Jira Test
type MockJiraClientJiraTest struct {
	CreateTicketFunc      func(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error)
	CreateChildTicketFunc func(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error)
	AddIssueLinkFunc      func(ctx context.Context, inwardKey, outwardKey, linkType string) error
}

func (m *MockJiraClientJiraTest) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	if m.CreateTicketFunc != nil {
		return m.CreateTicketFunc(ctx, projectKey, summary, description, issueType, labels)
	}
	return "TEST-1", nil
}

func (m *MockJiraClientJiraTest) CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
	if m.CreateChildTicketFunc != nil {
		return m.CreateChildTicketFunc(ctx, projectKey, summary, description, issueType, parentKey, labels)
	}
	return "TEST-2", nil
}

func (m *MockJiraClientJiraTest) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	if m.AddIssueLinkFunc != nil {
		return m.AddIssueLinkFunc(ctx, inwardKey, outwardKey, linkType)
	}
	return nil
}

// Mock Agent for Jira Test
type MockGenAgentJiraTest struct {
	Response string
}

func (m *MockGenAgentJiraTest) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockGenAgentJiraTest) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}
func (m *MockGenAgentJiraTest) SendImage(ctx context.Context, prompt string, imagePath string) (string, error) {
	return m.Response, nil
}

func TestGenerateTickets_Success(t *testing.T) {
	// JSON response from Agent
	jsonResp := `
[
  {
    "title": "ID:[EPIC-1] Epic Title",
    "description": "Epic Desc\nRepo: https://github.com/test/repo",
    "type": "Epic",
    "children": [
      {
        "title": "ID:[STORY-1] Story Title",
        "description": "Story Desc\nRepo: https://github.com/test/repo",
        "type": "Story",
        "children": []
      }
    ]
  }
]
`
	mockAgent := &MockGenAgentJiraTest{Response: jsonResp}

	mockJira := &MockJiraClientJiraTest{
		CreateTicketFunc: func(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
			if projectKey != "PROJ" {
				t.Errorf("Expected project PROJ, got %s", projectKey)
			}
			if issueType != "Epic" {
				t.Errorf("Expected Epic, got %s", issueType)
			}
			return "PROJ-1", nil
		},
		CreateChildTicketFunc: func(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
			if parentKey != "PROJ-1" {
				t.Errorf("Expected parent PROJ-1, got %s", parentKey)
			}
			if issueType != "Story" {
				t.Errorf("Expected Story, got %s", issueType)
			}
			return "PROJ-2", nil
		},
	}

	mapping, err := generateTickets(context.Background(), "spec", "PROJ", "", []string{"label"}, mockJira, mockAgent)
	if err != nil {
		t.Fatalf("generateTickets failed: %v", err)
	}

	if len(mapping) != 2 {
		t.Errorf("Expected 2 mapped IDs, got %d", len(mapping))
	}
	if mapping["EPIC-1"] != "PROJ-1" {
		t.Errorf("Expected EPIC-1 -> PROJ-1, got %s", mapping["EPIC-1"])
	}
	if mapping["STORY-1"] != "PROJ-2" {
		t.Errorf("Expected STORY-1 -> PROJ-2, got %s", mapping["STORY-1"])
	}
}

func TestGenerateTickets_ValidationFail(t *testing.T) {
	// Missing Repo URL in description and not provided in args
	jsonResp := `[{"title": "Epic", "description": "No repo here", "type": "Epic"}]`
	mockAgent := &MockGenAgentJiraTest{Response: jsonResp}
	mockJira := &MockJiraClientJiraTest{}

	_, err := generateTickets(context.Background(), "spec", "PROJ", "", nil, mockJira, mockAgent)
	if err == nil {
		t.Fatal("Expected validation error for missing Repo URL")
	}
	if !strings.Contains(err.Error(), "missing repository URL") {
		t.Errorf("Expected missing repo url error, got: %v", err)
	}
}

func TestGenerateTickets_WithBlockers(t *testing.T) {
	jsonResp := `
[
  {
    "title": "ID:[TASK-1] Task 1",
    "description": "Desc\nRepo: http://repo",
    "type": "Task"
  },
  {
    "title": "ID:[TASK-2] Task 2",
    "description": "Desc\nRepo: http://repo",
    "type": "Task",
    "blocked_by": ["ID:[TASK-1] Task 1"]
  }
]
`
	mockAgent := &MockGenAgentJiraTest{Response: jsonResp}

	linksCreated := 0
	mockJira := &MockJiraClientJiraTest{
		CreateTicketFunc: func(ctx context.Context, pk, sum, desc, it string, l []string) (string, error) {
			if strings.Contains(sum, "Task 1") {
				return "KEY-1", nil
			}
			return "KEY-2", nil
		},
		AddIssueLinkFunc: func(ctx context.Context, in, out, typ string) error {
			linksCreated++
			if in != "KEY-1" || out != "KEY-2" {
				t.Errorf("Expected link KEY-1 -> KEY-2, got %s -> %s", in, out)
			}
			return nil
		},
	}

	_, err := generateTickets(context.Background(), "spec", "PROJ", "", nil, mockJira, mockAgent)
	if err != nil {
		t.Fatalf("generateTickets failed: %v", err)
	}

	if linksCreated != 1 {
		t.Errorf("Expected 1 link created, got %d", linksCreated)
	}
}
