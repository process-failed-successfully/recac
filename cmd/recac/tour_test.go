package main

import (
	"context"
	"encoding/json"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestTourModel_Init(t *testing.T) {
	m := initialTourModel()
	cmd := m.Init()
	// cmd is a function, we can't easily check it equals loadTourCmd without reflection or executing it
	// But we can verify m.loading is true
	assert.True(t, m.loading)
	assert.NotNil(t, cmd)
}

func TestLoadTour_Fallback(t *testing.T) {
	// Reset config
	viper.Set("provider", "")
	viper.Set("model", "")

	msg := loadTourCmd()

	loadedMsg, ok := msg.(TourLoadedMsg)
	assert.True(t, ok)
	assert.NotNil(t, loadedMsg.err) // Expect error "AI provider not configured"
	assert.Equal(t, "AI provider not configured", loadedMsg.err.Error())
	assert.NotEmpty(t, loadedMsg.stops)
	assert.Equal(t, "README.md", loadedMsg.stops[0].Name)
}

func TestLoadTour_AI_Success(t *testing.T) {
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	mockAg := new(MockAgent)

	// Mock factory
	oldFactory := agentClientFactory
	defer func() { agentClientFactory = oldFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	stops := []TourStop{
		{Name: "Test Stop", Path: "test.go", Desc: "Description"},
	}
	jsonBytes, _ := json.Marshal(stops)

	// Match any prompt
	mockAg.On("Send", mock.Anything, mock.Anything).Return(string(jsonBytes), nil)

	msg := loadTourCmd()

	loadedMsg, ok := msg.(TourLoadedMsg)
	assert.True(t, ok)
	assert.Nil(t, loadedMsg.err)
	assert.Equal(t, 1, len(loadedMsg.stops))
	assert.Equal(t, "Test Stop", loadedMsg.stops[0].Name)
}

func TestLoadTour_AI_Failure(t *testing.T) {
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	mockAg := new(MockAgent)

	oldFactory := agentClientFactory
	defer func() { agentClientFactory = oldFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAg, nil
	}

	mockAg.On("Send", mock.Anything, mock.Anything).Return("", assert.AnError)

	msg := loadTourCmd()

	loadedMsg, ok := msg.(TourLoadedMsg)
	assert.True(t, ok)
	assert.NotNil(t, loadedMsg.err)
	// It should return fallback stops even on error
	assert.NotEmpty(t, loadedMsg.stops)
	assert.Equal(t, "README.md", loadedMsg.stops[0].Name)
}
