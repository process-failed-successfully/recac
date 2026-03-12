package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExportPipelineToYAML_Success(t *testing.T) {
	maxRetries := 2
	jobs := []JobInfo{
		{
			ID: "job-1",
			WorkItem: WorkItem{
				Summary:          "Build App",
				Description:      "npm run build",
				RepoURL:          "https://github.com/org/repo.git",
				EnvVars:          map[string]string{"NODE_ENV": "production"},
				Tags:             []string{"build"},
				Priority:         10,
				Timeout:          30 * time.Minute,
				Delay:            5 * time.Minute,
				ConcurrencyGroup: "group-1",
				CancelInProgress: true,
				AgentProvider:    "openai",
				AgentModel:       "gpt-4",
				MaxRetries:       &maxRetries,
			},
		},
		{
			ID: "job-2",
			WorkItem: WorkItem{
				Summary:   "Test App",
				DependsOn: []string{"job-1"},
			},
		},
	}

	yamlData, err := ExportPipelineToYAML("my-pipeline", jobs)
	require.NoError(t, err)

	var p Pipeline
	err = yaml.Unmarshal(yamlData, &p)
	require.NoError(t, err)

	assert.Equal(t, "my-pipeline", p.Name)
	assert.Len(t, p.Jobs, 2)

	j1 := p.Jobs["job-1"]
	assert.Equal(t, "Build App", j1.Summary)
	assert.Equal(t, "npm run build", j1.Description)
	assert.Equal(t, "https://github.com/org/repo.git", j1.RepoURL)
	assert.Equal(t, map[string]string{"NODE_ENV": "production"}, j1.EnvVars)
	assert.Equal(t, []string{"build"}, j1.Tags)
	assert.Equal(t, 10, j1.Priority)
	assert.Equal(t, "30m0s", j1.Timeout)
	assert.Equal(t, "5m0s", j1.Delay)
	assert.Equal(t, "group-1", j1.ConcurrencyGroup)
	assert.True(t, j1.CancelInProgress)
	assert.Equal(t, "openai", j1.AgentProvider)
	assert.Equal(t, "gpt-4", j1.AgentModel)
	require.NotNil(t, j1.MaxRetries)
	assert.Equal(t, 2, *j1.MaxRetries)
	assert.Empty(t, j1.DependsOn)

	j2 := p.Jobs["job-2"]
	assert.Equal(t, "Test App", j2.Summary)
	assert.Equal(t, []string{"job-1"}, j2.DependsOn)
}

func TestExportPipelineToYAML_Empty(t *testing.T) {
	yamlData, err := ExportPipelineToYAML("empty-pipeline", []JobInfo{})
	require.NoError(t, err)

	var p Pipeline
	err = yaml.Unmarshal(yamlData, &p)
	require.NoError(t, err)

	assert.Equal(t, "empty-pipeline", p.Name)
	assert.Empty(t, p.Jobs)
}
