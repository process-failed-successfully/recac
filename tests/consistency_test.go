package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Orchestrator struct {
		MetricsPort int `yaml:"metrics_port"`
	} `yaml:"orchestrator"`
}

type HelmValues struct {
	Config struct {
		MetricsPort int `yaml:"metricsPort"`
	} `yaml:"config"`
	Service struct {
		Port int `yaml:"port"`
	} `yaml:"service"`
}

func TestMetricsPortConsistency(t *testing.T) {
	// Find project root
	// Assuming tests/consistency_test.go is 1 level deep from root
	root := ".."

	configPath := filepath.Join(root, "cmd/orchestrator/config.yaml")
	valuesPath := filepath.Join(root, "deploy/helm/recac/values.yaml")

	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config.yaml: %v", err)
	}

	valuesData, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatalf("Failed to read values.yaml: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to unmarshal config.yaml: %v", err)
	}

	var values HelmValues
	if err := yaml.Unmarshal(valuesData, &values); err != nil {
		t.Fatalf("Failed to unmarshal values.yaml: %v", err)
	}

	t.Logf("config.yaml orchestrator.metrics_port: %d", config.Orchestrator.MetricsPort)
	t.Logf("values.yaml config.metricsPort: %d", values.Config.MetricsPort)
	t.Logf("values.yaml service.port: %d", values.Service.Port)

	assert.Equal(t, 2112, config.Orchestrator.MetricsPort, "config.yaml orchestrator.metrics_port should be 2112")
	assert.Equal(t, config.Orchestrator.MetricsPort, values.Config.MetricsPort, "Helm values.yaml metricsPort should match config.yaml orchestrator.metrics_port")
	assert.Equal(t, config.Orchestrator.MetricsPort, values.Service.Port, "Helm values.yaml service.port should match config.yaml orchestrator.metrics_port")
}
