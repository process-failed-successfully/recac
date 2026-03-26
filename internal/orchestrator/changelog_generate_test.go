package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/spf13/viper"

	"recac/internal/agent"
)

type customMockAgent struct {
	*agent.MockAgent
	SentPrompts []string
	Responses   []string
	respIdx     int
}

func (m *customMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	// Call underlying MockAgent to check for forced error
	if _, err := m.MockAgent.Send(ctx, prompt); err != nil {
		return "", err
	}

	m.SentPrompts = append(m.SentPrompts, prompt)
	if m.respIdx < len(m.Responses) {
		resp := m.Responses[m.respIdx]
		m.respIdx++
		return resp, nil
	}
	return "default mock response", nil
}

func TestGenerateChangelog(t *testing.T) {
	// Mock the agent
	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := &customMockAgent{
		MockAgent: agent.NewMockAgent(),
	}

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return mockAgent, nil
	}

	viper.Set("orchestrator.agent_provider", "mock")
	viper.Set("orchestrator.agent_model", "mock-model")

	orch := New(nil, nil, 1*time.Second)

	job1 := JobInfo{
		ID:      "JOB-1",
		Summary: "Feature A",
		Status:  "Completed",
		WorkItem: WorkItem{
			Tags: []string{"backend"},
		},
	}
	job2 := JobInfo{
		ID:      "JOB-2",
		Summary: "Bug Fix B",
		Status:  "Failed", // Should be ignored
	}
	job3 := JobInfo{
		ID:      "JOB-3",
		Summary: "Refactor C",
		Status:  "Completed",
		WorkItem: WorkItem{
			Tags: []string{"frontend"},
		},
	}

	orch.mu.Lock()
	orch.completedJobs = []JobInfo{job1, job2, job3}
	orch.mu.Unlock()

	t.Run("GenerateAllCompleted", func(t *testing.T) {
		mockAgent.SentPrompts = nil
		mockAgent.respIdx = 0
		mockAgent.Responses = []string{"Changelog All"}

		res, err := GenerateChangelog(context.Background(), orch, "", "", "", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "Changelog All", res)

		// Verify prompt included JOB-1 and JOB-3, but not JOB-2
		assert.Len(t, mockAgent.SentPrompts, 1)
		prompt := mockAgent.SentPrompts[0]
		assert.Contains(t, prompt, "JOB-1")
		assert.Contains(t, prompt, "JOB-3")
		assert.NotContains(t, prompt, "JOB-2")
	})

	t.Run("GenerateWithTagFilter", func(t *testing.T) {
		mockAgent.SentPrompts = nil
		mockAgent.respIdx = 0
		mockAgent.Responses = []string{"Changelog Tag"}

		res, err := GenerateChangelog(context.Background(), orch, "backend", "", "", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "Changelog Tag", res)

		assert.Len(t, mockAgent.SentPrompts, 1)
		prompt := mockAgent.SentPrompts[0]
		assert.Contains(t, prompt, "JOB-1")
		assert.NotContains(t, prompt, "JOB-3")
	})

	t.Run("GenerateWithMatchFilter", func(t *testing.T) {
		mockAgent.SentPrompts = nil
		mockAgent.respIdx = 0
		mockAgent.Responses = []string{"Changelog Match"}

		res, err := GenerateChangelog(context.Background(), orch, "", "Refactor", "", "", "")
		assert.NoError(t, err)
		assert.Equal(t, "Changelog Match", res)

		assert.Len(t, mockAgent.SentPrompts, 1)
		prompt := mockAgent.SentPrompts[0]
		assert.Contains(t, prompt, "JOB-3")
		assert.NotContains(t, prompt, "JOB-1")
	})

	t.Run("NoJobsMatch", func(t *testing.T) {
		mockAgent.SentPrompts = nil
		mockAgent.respIdx = 0

		res, err := GenerateChangelog(context.Background(), orch, "nonexistent", "", "", "", "")
		assert.NoError(t, err)
		assert.Contains(t, res, "No completed jobs found matching the criteria")
		assert.Len(t, mockAgent.SentPrompts, 0) // Should not call AI
	})
}
