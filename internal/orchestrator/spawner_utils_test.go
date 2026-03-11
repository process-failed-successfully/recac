package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectAgentEnvVars_PrioritizesWorkItem(t *testing.T) {
	// Define default values
	defaultProvider := "default-provider"
	defaultModel := "default-model"

	// Scenario 1: WorkItem has specific provider and model
	itemWithSpecifics := WorkItem{
		ID:            "JOB-1",
		AgentProvider: "workitem-provider",
		AgentModel:    "workitem-model",
	}

	env1 := collectAgentEnvVars(itemWithSpecifics, defaultProvider, defaultModel)
	assert.Equal(t, "workitem-provider", env1["RECAC_PROVIDER"])
	assert.Equal(t, "workitem-model", env1["RECAC_MODEL"])

	// Scenario 2: WorkItem is missing specific provider and model, should fall back to defaults
	itemWithoutSpecifics := WorkItem{
		ID: "JOB-2",
	}

	env2 := collectAgentEnvVars(itemWithoutSpecifics, defaultProvider, defaultModel)
	assert.Equal(t, "default-provider", env2["RECAC_PROVIDER"])
	assert.Equal(t, "default-model", env2["RECAC_MODEL"])

	// Scenario 3: WorkItem has specific provider but missing model
	itemPartial := WorkItem{
		ID:            "JOB-3",
		AgentProvider: "workitem-provider",
	}

	env3 := collectAgentEnvVars(itemPartial, defaultProvider, defaultModel)
	assert.Equal(t, "workitem-provider", env3["RECAC_PROVIDER"])
	assert.Equal(t, "default-model", env3["RECAC_MODEL"])
}
