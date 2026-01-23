package main

import (
	"context"
	"testing"

	"recac/internal/agent"
	"github.com/stretchr/testify/assert"
)

func TestGenerateArchitecture(t *testing.T) {
	mockAgent := agent.NewMockAgent()

	// Valid JSON response
	jsonResp := "```json\n{\"architecture.yaml\": \"content\", \"contracts/api.yaml\": \"api\"}\n```"
	mockAgent.SetResponse(jsonResp)

	files, err := generateArchitecture(context.Background(), mockAgent, "spec")
	assert.NoError(t, err)
	assert.Equal(t, "content", files["architecture.yaml"])
	assert.Equal(t, "api", files["contracts/api.yaml"])

	// Valid JSON response without markdown
	jsonRespNoMarkdown := "{\"architecture.yaml\": \"content2\"}"
	mockAgent.SetResponse(jsonRespNoMarkdown)

	files, err = generateArchitecture(context.Background(), mockAgent, "spec")
	assert.NoError(t, err)
	assert.Equal(t, "content2", files["architecture.yaml"])

	// Invalid JSON
	mockAgent.SetResponse("not json")
	_, err = generateArchitecture(context.Background(), mockAgent, "spec")
	assert.Error(t, err)
}
