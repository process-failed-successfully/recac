package main

import (
	"bytes"
	"context"
	"testing"

	"recac/internal/architecture"
	"github.com/stretchr/testify/assert"
)

type MockJiraClientStatus struct {
	SearchIssuesFunc func(ctx context.Context, jql string) ([]map[string]interface{}, error)
}

func (m *MockJiraClientStatus) CreateTicket(ctx context.Context, projectKey, summary, description, issueType string, labels []string) (string, error) {
	return "TEST-1", nil
}
func (m *MockJiraClientStatus) CreateChildTicket(ctx context.Context, projectKey, summary, description, issueType, parentKey string, labels []string) (string, error) {
	return "TEST-2", nil
}
func (m *MockJiraClientStatus) AddIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	return nil
}
func (m *MockJiraClientStatus) SearchIssues(ctx context.Context, jql string) ([]map[string]interface{}, error) {
	if m.SearchIssuesFunc != nil {
		return m.SearchIssuesFunc(ctx, jql)
	}
	return nil, nil
}

func TestCalculateAndPrintStatus(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{ID: "USER-SERVICE", Type: "service"},
			{ID: "DB", Type: "database"},
		},
	}

	mockClient := &MockJiraClientStatus{
		SearchIssuesFunc: func(ctx context.Context, jql string) ([]map[string]interface{}, error) {
			if jql == "project = \"PROJ\" AND summary ~ \"ID:[USER-SERVICE]*\"" {
				return []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary": "ID:[USER-SERVICE] User Service",
							"status": map[string]interface{}{
								"name": "Done",
							},
						},
					},
				}, nil
			}
			if jql == "project = \"PROJ\" AND summary ~ \"ID:[DB]*\"" {
				return []map[string]interface{}{
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary": "ID:[DB] Database",
							"status": map[string]interface{}{
								"name": "In Progress",
							},
						},
					},
				}, nil
			}
			return nil, nil
		},
	}

	var buf bytes.Buffer
	err := calculateAndPrintStatus(context.Background(), mockClient, arch, "PROJ", &buf)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "USER-SERVICE")
	assert.Contains(t, output, "PROJ-1")
	assert.Contains(t, output, "Done")
	assert.Contains(t, output, "100%")

	assert.Contains(t, output, "DB")
	assert.Contains(t, output, "PROJ-2")
	assert.Contains(t, output, "In Progress")
	assert.Contains(t, output, "50%")

	assert.Contains(t, output, "TOTAL")
	assert.Contains(t, output, "50.0%")
}
