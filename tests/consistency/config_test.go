package consistency

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type Config struct {
	MetricsPort int `yaml:"metrics_port"`
}

type Values struct {
	Service struct {
		Port int `yaml:"port"`
	} `yaml:"service"`
	Config struct {
		MetricsPort int `yaml:"metricsPort"`
	} `yaml:"config"`
}

func TestDockerK8sMetricsPortConsistency(t *testing.T) {
	// Find the root of the repository
	// Assuming this test runs from tests/consistency/
	rootDir := "../../"

	// Read config.yaml (Docker/Local default)
	configPath := filepath.Join(rootDir, "config.yaml")
	configBytes, err := os.ReadFile(configPath)
	require.NoError(t, err, "Failed to read config.yaml")

	var config Config
	err = yaml.Unmarshal(configBytes, &config)
	require.NoError(t, err, "Failed to parse config.yaml")

	// Read values.yaml (K8s default)
	valuesPath := filepath.Join(rootDir, "deploy/helm/recac/values.yaml")
	valuesBytes, err := os.ReadFile(valuesPath)
	require.NoError(t, err, "Failed to read values.yaml")

	var values Values
	err = yaml.Unmarshal(valuesBytes, &values)
	require.NoError(t, err, "Failed to parse values.yaml")

	// Verify Consistency
	t.Run("Metrics Port Consistency", func(t *testing.T) {
		assert.Equal(t, config.MetricsPort, values.Config.MetricsPort, "Docker (config.yaml) and K8s (values.yaml) metrics ports mismatch")
		assert.Equal(t, values.Config.MetricsPort, values.Service.Port, "K8s service port does not match metrics port")
		assert.Equal(t, 2112, config.MetricsPort, "Expected metrics port to be 2112")
	})
}
