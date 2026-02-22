package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGenerateTourStops_Success(t *testing.T) {
	mockAgent := new(MockAgent)
	ctx := context.Background()

	// Mock AI Response
	expectedStops := []TourStop{
		{Path: "main.go", Desc: "Entry point"},
		{Path: "pkg/lib.go", Desc: "Library code"},
	}
	jsonBytes, _ := json.Marshal(expectedStops)

	// The prompt will contain the file tree, so we use mock.Anything for the prompt string
	mockAgent.On("Send", ctx, mock.Anything).Return(string(jsonBytes), nil)

	stops, err := GenerateTourStops(ctx, mockAgent)

	assert.NoError(t, err)
	assert.Len(t, stops, 2)
	assert.Equal(t, "main.go", stops[0].Path)
	assert.Equal(t, "Entry point", stops[0].Desc)

	mockAgent.AssertExpectations(t)
}

func TestGenerateTourStops_JSONBlock(t *testing.T) {
	mockAgent := new(MockAgent)
	ctx := context.Background()

	// Mock AI Response wrapped in markdown code block
	expectedStops := []TourStop{
		{Path: "main.go", Desc: "Entry point"},
	}
	jsonBytes, _ := json.Marshal(expectedStops)
	resp := "Here is the plan:\n```json\n" + string(jsonBytes) + "\n```"

	mockAgent.On("Send", ctx, mock.Anything).Return(resp, nil)

	stops, err := GenerateTourStops(ctx, mockAgent)

	assert.NoError(t, err)
	assert.Len(t, stops, 1)
	assert.Equal(t, "main.go", stops[0].Path)

	mockAgent.AssertExpectations(t)
}

func TestGenerateTourStops_AgentFailure(t *testing.T) {
	mockAgent := new(MockAgent)
	ctx := context.Background()

	mockAgent.On("Send", ctx, mock.Anything).Return("", assert.AnError)

	_, err := GenerateTourStops(ctx, mockAgent)
	assert.Error(t, err)
}

func TestGenerateTourStops_InvalidJSON(t *testing.T) {
	mockAgent := new(MockAgent)
	ctx := context.Background()

	mockAgent.On("Send", ctx, mock.Anything).Return("Not JSON", nil)

	_, err := GenerateTourStops(ctx, mockAgent)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse JSON")
}

func TestTourCmd_Setup(t *testing.T) {
	assert.Equal(t, "tour", tourCmd.Use)
	assert.NotEmpty(t, tourCmd.Short)
	assert.NotEmpty(t, tourCmd.Long)
}
