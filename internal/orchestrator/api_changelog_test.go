package orchestrator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"io"
	"log/slog"

	"github.com/stretchr/testify/assert"
	"github.com/spf13/viper"
	"recac/internal/agent"
)

func TestAPI_GenerateChangelog(t *testing.T) {
	orch := New(nil, nil, 1*time.Second)

	job := JobInfo{
		ID:      "JOB-1",
		Summary: "Feature A",
		Status:  "Completed",
	}
	orch.mu.Lock()
	orch.completedJobs = append(orch.completedJobs, job)
	orch.mu.Unlock()

	originalNewAgentFunc := newAgentFunc
	defer func() { newAgentFunc = originalNewAgentFunc }()

	mockAgent := &customMockAgent{
		MockAgent: agent.NewMockAgent(),
	}
	newAgentFunc = func(provider, apiKey, model, baseURL, systemPrompt string) (agent.Agent, error) {
		return mockAgent, nil
	}

	viper.Set("api_key", "test-key")
	viper.Set("orchestrator.agent_provider", "mock")
	viper.Set("orchestrator.agent_model", "mock-model")

	mux := http.NewServeMux()
	l := slog.New(slog.NewTextHandler(io.Discard, nil))
	RegisterAPI(mux, orch, l, context.Background())

	t.Run("Success", func(t *testing.T) {
		mockAgent.SentPrompts = nil
		mockAgent.respIdx = 0
		mockAgent.Responses = []string{"# AI Changelog"}

		req := httptest.NewRequest(http.MethodGet, "/changelog/generate", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		res := w.Result()
		assert.Equal(t, http.StatusOK, res.StatusCode)

		var data map[string]string
		err := json.NewDecoder(res.Body).Decode(&data)
		assert.NoError(t, err)
		assert.Equal(t, "# AI Changelog", data["changelog"])
	})

	t.Run("AIError", func(t *testing.T) {
		mockAgent.SetError(assert.AnError)
		defer mockAgent.SetError(nil)

		req := httptest.NewRequest(http.MethodGet, "/changelog/generate", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		res := w.Result()
		assert.Equal(t, http.StatusInternalServerError, res.StatusCode)

		bodyBytes, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(bodyBytes), "Failed to generate changelog")
	})
}
