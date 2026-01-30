package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestTopologyGraphGeneration(t *testing.T) {
	// 1. Define input DockerCompose struct directly
	// We mimic what unmarshalling would produce to test the graph generation logic

	// Complex case with map depends_on
	compose := DockerCompose{
		Version: "3.9",
		Services: map[string]Service{
			"web": {
				Image: "nginx",
				Ports: []string{"80:80"},
				DependsOn: []interface{}{"db", "redis"},
			},
			"db": {
				Image: "postgres",
			},
			"redis": {
				Image: "redis",
			},
			"worker": {
				Image: "worker",
				DependsOn: map[string]interface{}{
					"db": map[string]interface{}{"condition": "healthy"},
				},
			},
		},
	}

	// 2. Generate Graph
	output := generateTopologyGraph(compose)

	// 3. Assertions
	assert.Contains(t, output, "graph TD")

	// Check nodes
	assert.Contains(t, output, `web["web<br/>(80:80)"]`)
	assert.Contains(t, output, `db["db"]`)
	assert.Contains(t, output, `redis["redis"]`)
	assert.Contains(t, output, `worker["worker"]`)

	// Check dependencies (arrows)
	assert.Contains(t, output, "db --> web")
	assert.Contains(t, output, "redis --> web")
	assert.Contains(t, output, "db --> worker")
}

func TestTopologyParsing(t *testing.T) {
	// Verify that YAML unmarshaling works as expected, especially depends_on
	content := `
services:
  web:
    depends_on:
      - db
  worker:
    depends_on:
      db:
        condition: healthy
`
	var compose DockerCompose
	err := yaml.Unmarshal([]byte(content), &compose)
	assert.NoError(t, err)

	// Check web depends_on (list)
	webDeps := parseDependsOn(compose.Services["web"].DependsOn)
	assert.Contains(t, webDeps, "db")

	// Check worker depends_on (map)
	workerDeps := parseDependsOn(compose.Services["worker"].DependsOn)
	assert.Contains(t, workerDeps, "db")
}
