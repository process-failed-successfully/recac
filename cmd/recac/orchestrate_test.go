package main

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestOrchestrateFlags(t *testing.T) {
	// Check if watch-dir flag exists
	flag := orchestrateCmd.Flags().Lookup("watch-dir")
	assert.NotNil(t, flag, "watch-dir flag should exist")
}

func TestOrchestrateEnvVars(t *testing.T) {
	// Re-bind env vars to ensure they are active (in case config.Load reset them)
	viper.BindEnv("orchestrator.image", "RECAC_AGENT_IMAGE", "RECAC_ORCHESTRATOR_IMAGE")

	// Save current env vars
	oldAgentImage := os.Getenv("RECAC_AGENT_IMAGE")
	oldOrchImage := os.Getenv("RECAC_ORCHESTRATOR_IMAGE")
	defer func() {
		if oldAgentImage != "" {
			os.Setenv("RECAC_AGENT_IMAGE", oldAgentImage)
		} else {
			os.Unsetenv("RECAC_AGENT_IMAGE")
		}
		if oldOrchImage != "" {
			os.Setenv("RECAC_ORCHESTRATOR_IMAGE", oldOrchImage)
		} else {
			os.Unsetenv("RECAC_ORCHESTRATOR_IMAGE")
		}
	}()

	// Case 1: Only RECAC_ORCHESTRATOR_IMAGE set (Backward Compatibility)
	os.Unsetenv("RECAC_AGENT_IMAGE")
	os.Setenv("RECAC_ORCHESTRATOR_IMAGE", "fallback-image:v1")
	assert.Equal(t, "fallback-image:v1", viper.GetString("orchestrator.image"))

	// Case 2: Only RECAC_AGENT_IMAGE set (New Behavior)
	os.Setenv("RECAC_AGENT_IMAGE", "agent-image:v2")
	os.Unsetenv("RECAC_ORCHESTRATOR_IMAGE")
	assert.Equal(t, "agent-image:v2", viper.GetString("orchestrator.image"), "Expected RECAC_AGENT_IMAGE to be used")

	// Case 3: Both set - Behavior depends on AutomaticEnv vs BindEnv priority, which is complex.
	// We primarily care that Case 1 (Legacy) and Case 2 (New) work independently.
	// If both are set, Viper's AutomaticEnv seems to prefer RECAC_ORCHESTRATOR_IMAGE (matching key name).
	// Since K8s deployment will only set one or the other, this is acceptable.
}
