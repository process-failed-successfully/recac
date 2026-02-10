package agent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockAgent_GitAuth_Injection(t *testing.T) {
	agent := NewMockAgent()
	prompt := "ROLE - INITIALIZER AGENT\nRepo: https://github.com/example/repo.git"

	response, err := agent.Send(context.Background(), prompt)
	assert.NoError(t, err)

	// Verify that the script contains the auth injection logic using POSIX parameter expansion
	assert.Contains(t, response, "if [ -n \"$GITHUB_API_KEY\" ]")

	// Expect the specific POSIX variable manipulation
	assert.Contains(t, response, "CLEAN_URL=${REPO_URL#https://}")
	assert.Contains(t, response, "AUTH_URL=\"https://x-access-token:${GITHUB_API_KEY}@${CLEAN_URL}\"")

	// Verify that the fallback logic is also present
	assert.Contains(t, response, "else")
	assert.Contains(t, response, "Initializing local repo")
}
