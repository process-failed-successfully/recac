package runner

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSessionWithConfig(t *testing.T) {
	workspace := t.TempDir()
	project := "test-project"
	provider := "test-provider"
	model := "test-model"

	// We can pass nil for dbStore as we are just testing struct initialization logic
	// and ensuring no panics occur.
	session := NewSessionWithConfig(workspace, project, provider, model, nil)

	assert.NotNil(t, session)
	assert.Equal(t, workspace, session.Workspace)
	assert.Equal(t, project, session.Project)
	assert.Equal(t, provider, session.AgentProvider)
	assert.Equal(t, model, session.AgentModel)
	assert.Nil(t, session.DBStore)
	assert.Equal(t, "app_spec.txt", session.SpecFile)
	assert.Equal(t, 20, session.MaxIterations)
	assert.Equal(t, 5, session.ManagerFrequency)
	assert.False(t, session.OwnsDB)
	assert.NotNil(t, session.StateManager)
	assert.NotNil(t, session.Scanner)
	assert.NotNil(t, session.Notifier)
	assert.NotNil(t, session.Logger)

	// Check agent state file path
	expectedStateFile := filepath.Join(workspace, ".agent_state.json")
	assert.Equal(t, expectedStateFile, session.AgentStateFile)
}

func TestNewSessionWithConfig_EmptyProject(t *testing.T) {
	workspace := t.TempDir()
	session := NewSessionWithConfig(workspace, "", "prov", "mod", nil)

	assert.Equal(t, "unknown", session.Project)
}
