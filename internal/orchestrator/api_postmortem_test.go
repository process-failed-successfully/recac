package orchestrator

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"context"
	"fmt"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAPIGeneratePostmortem_Success(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(nil, fmt.Errorf("no logs"))

	orch := New(&MockPoller{}, mockSpawner, 1*time.Second)

	orch.completedJobs = []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Failed",
			Summary:   "Build failed",
			Error:     "exit status 1",
			StartTime: time.Now(),
		},
	}

	oldNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		assert.Equal(t, "custom-provider", provider)
		assert.Equal(t, "custom-model", model)
		return &mockPostmortemAgent{
			response: "# Custom Postmortem",
		}, nil
	}

	req, _ := http.NewRequest("GET", "/postmortem/generate?provider=custom-provider&model=custom-model", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var result map[string]string
	err := json.NewDecoder(rr.Body).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "# Custom Postmortem", result["postmortem"])
}

func TestAPIGeneratePostmortem_AgentFailure(t *testing.T) {
	mockSpawner := new(MockSpawner)
	mockSpawner.On("GetLogs", mock.Anything, "JOB-1").Return(nil, fmt.Errorf("no logs"))

	orch := New(&MockPoller{}, mockSpawner, 1*time.Second)

	orch.completedJobs = []JobInfo{
		{
			ID:        "JOB-1",
			Status:    "Failed",
		},
	}

	oldNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = oldNewAgentFunc }()

	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return &mockPostmortemAgent{
			err: fmt.Errorf("AI error"),
		}, nil
	}

	req, _ := http.NewRequest("GET", "/postmortem/generate", nil)
	rr := httptest.NewRecorder()

	mux := http.NewServeMux()
	RegisterAPI(mux, orch, nil, context.Background())
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "Failed to generate postmortem")
}
