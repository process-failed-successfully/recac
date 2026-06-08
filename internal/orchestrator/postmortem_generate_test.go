package orchestrator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPostmortemAgent struct {
	response string
	err      error
}

func (m *mockPostmortemAgent) Send(ctx context.Context, prompt string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockPostmortemAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	callback(m.response)
	return m.response, nil
}

func (m *mockPostmortemAgent) Name() string {
	return "mock"
}

func (m *mockPostmortemAgent) Close() error {
	return nil
}

func TestGeneratePostmortem_Success(t *testing.T) {
	// 1. Setup orchestrator
	mockSpawner := new(MockSpawner)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(nil, fmt.Errorf("no logs"))
	mockSpawner.On("GetLogs", mock.Anything, "JOB-3").Return(nil, fmt.Errorf("no logs"))

	orch := New(&MockPoller{}, mockSpawner, 1*time.Second)

	// 2. Add some failed jobs
	orch.completedJobs = []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Failed",
			Summary:   "Build failure",
			Error:     "exit status 1",
			StartTime: time.Now(),
			WorkItem:  WorkItem{Tags: []string{"backend"}},
		},
		{
			ID:        "JOB-2",
			Status:    "Completed",
			Summary:   "Success job",
			StartTime: time.Now(),
		},
		{
			ID:        "JOB-3",
			Status:    "Failed",
			Summary:   "Test failure",
			Error:     "tests failed",
			StartTime: time.Now(),
			WorkItem:  WorkItem{Tags: []string{"frontend"}},
		},
	}

	// 3. Mock Agent Factory
	oldNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockPostmortemAgent{
			response: "# Postmortem Report\n\nAll tests failed.",
		}, nil
	}

	// 4. Run GeneratePostmortem
	ctx := context.Background()
	res, err := GeneratePostmortem(ctx, orch, "", "", "mock", "mock-model", "test-key")

	// 5. Verify
	assert.NoError(t, err)
	assert.Contains(t, res, "Postmortem Report")
}

func TestGeneratePostmortem_NoFailedJobs(t *testing.T) {
	mockSpawner := new(MockSpawner)
	orch := New(&MockPoller{}, mockSpawner, 1*time.Second)

	orch.completedJobs = []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Completed",
			Summary:   "Build success",
			StartTime: time.Now(),
		},
	}

	ctx := context.Background()
	res, err := GeneratePostmortem(ctx, orch, "", "", "mock", "mock-model", "test-key")

	assert.NoError(t, err)
	assert.Equal(t, "No failed jobs found to generate a postmortem.", res)
}

func TestGeneratePostmortem_AgentFailure(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(nil, fmt.Errorf("no logs"))
	orch := New(&MockPoller{}, mockSpawner, 1*time.Second)

	orch.completedJobs = []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Failed",
			Summary:   "Build failure",
			Error:     "exit status 1",
			StartTime: time.Now(),
		},
	}

	oldNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockPostmortemAgent{
			err: fmt.Errorf("AI service unavailable"),
		}, nil
	}

	ctx := context.Background()
	_, err := GeneratePostmortem(ctx, orch, "", "", "mock", "mock-model", "test-key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get postmortem from AI: AI service unavailable")
}

func TestGeneratePostmortem_Filters(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-2").Return(nil, fmt.Errorf("no logs"))

	orch := New(&MockPoller{}, mockSpawner, 1*time.Second)

	orch.completedJobs = []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Failed",
			Summary:   "Build failure",
			Error:     "exit status 1",
			StartTime: time.Now(),
			WorkItem:  WorkItem{Tags: []string{"backend"}},
		},
		{
			ID:        "JOB-2",
			Status:    "Failed",
			Summary:   "Test failure",
			Error:     "exit status 2",
			StartTime: time.Now(),
			WorkItem:  WorkItem{Tags: []string{"frontend"}},
		},
	}

	// Mock Agent
	oldNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldNewAgentFunc }()
	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockPostmortemAgent{response: "Report"}, nil
	}

	ctx := context.Background()

	// Filter by tag
	res, err := GeneratePostmortem(ctx, orch, "frontend", "", "mock", "mock-model", "test-key")
	assert.NoError(t, err)
	assert.Equal(t, "Report", res)
}

// TestGeneratePostmortem_InvalidRegex is removed since regex matching was replaced by utils.ContainsFold.
